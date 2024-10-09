// digest course make

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Shimi9999/gobms"
	"github.com/mattn/go-lsd"
)

func main() {
	var (
		all = flag.String("all", "", "All matched csv path")
	)
	flag.Parse()

	if *all == "" {
		if flag.NArg() < 2 {
			fmt.Println("Usage: dcmake {<bms rootdir path> <rank csv path> | -all <all matched csv path>}")
			os.Exit(1)
		}

		dirpath := flag.Arg(0)
		rankpath := flag.Arg(1)
		if _, err := os.Stat(dirpath); err != nil {
			fmt.Println("Bms rootdir path Error: " + err.Error())
			os.Exit(1)
		}
		if _, err := os.Stat(rankpath); err != nil {
			fmt.Println("Rank csv path Error: " + err.Error())
			os.Exit(1)
		}

		bmsdirs := make([]gobms.BmsDirectory, 0)
		err := gobms.FindBmsInDirectory(dirpath, &bmsdirs)
		if err != nil {
			fmt.Println("FindBmsInDirectory Error: " + err.Error())
			os.Exit(1)
		}

		entries, err := loadRankCsv(rankpath)
		if err != nil {
			fmt.Println("loadRankCsv Error: " + err.Error())
			os.Exit(1)
		}

		rankedDirs, unmatchEntries, remaingingBmsdirs := matchEntriesByInfo(bmsdirs, entries)
		fmt.Printf("match %d, unmatch %d, remaining directories %d\n", len(entries)-len(unmatchEntries), len(unmatchEntries), len(remaingingBmsdirs))

		if len(rankedDirs) > 0 && len(unmatchEntries) == 0 {
			rankedPathes := make([]string, len(rankedDirs))
			for i, dir := range rankedDirs {
				rankedPathes[i] = dir.Path
			}
			if err := outputCource(rankedPathes); err != nil {
				fmt.Println("outputCource Error: " + err.Error())
				os.Exit(1)
			}
		} else {
			if err := outputCsv(entries, rankedDirs, unmatchEntries, remaingingBmsdirs); err != nil {
				fmt.Println("outputCsv Error: " + err.Error())
				os.Exit(1)
			}
		}
	} else {
		bmsDirPaths, err := loadAllMatchedCsv(*all)
		if err != nil {
			fmt.Println("loadAllMatchedCsv Error: " + err.Error())
			os.Exit(1)
		}

		if err := outputCource(bmsDirPaths); err != nil {
			fmt.Println("outputCource Error: " + err.Error())
			os.Exit(1)
		}
	}
}

type bmsEntryInfo struct {
	title  string
	genre  string
	artist string
}

// csv col1:Title col2:Genre col3:Artist
func loadRankCsv(path string) ([]bmsEntryInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("csv open: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	var line []string
	var entries []bmsEntryInfo

	for {
		line, err = reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("csv read: %w", err)
		} else {
			b := bmsEntryInfo{line[0], line[1], line[2]}
			entries = append(entries, b)
		}
	}

	return entries, nil
}

type unmatchEntry struct {
	index     int
	entryInfo bmsEntryInfo
}

func matchEntriesByInfo(bmsdirs []gobms.BmsDirectory, entries []bmsEntryInfo) ([]gobms.BmsDirectory, []unmatchEntry, []gobms.BmsDirectory) {
	rankedDirs := make([]gobms.BmsDirectory, len(entries))
	unmatchDirs := make([]gobms.BmsDirectory, len(bmsdirs))
	copy(unmatchDirs, bmsdirs)
	unmatchEntries := []unmatchEntry{}

	type matchedDirDistances struct {
		titleDistance  float64
		genreDistance  float64
		artistDistance float64
	}

	// タイトル、ジャンル、アーティストの一致度合でマッチング
	// 標準化したレーベンシュタイン距離を利用
	for i, entry := range entries {
		maxMatchLevel := 0
		matchDirIndex := -1
		for j, dir := range unmatchDirs {
			if dir.Name == "" || entry.title == "" || len(dir.BmsDataSet) == 0 {
				continue
			}

			matchLevel := 0
			pureDirName := gobms.RemoveSuffixChartName(dir.Name)
			matchedDir := matchedDirDistances{
				normalizedLevenshteinDistance(pureDirName, entry.title),
				normalizedLevenshteinDistance(dir.BmsDataSet[0].Genre, entry.genre),
				normalizedLevenshteinDistance(dir.BmsDataSet[0].Artist, entry.artist),
			}

			if matchedDir.titleDistance+matchedDir.genreDistance+matchedDir.artistDistance == 0 { // 完全一致
				matchDirIndex = j
				break
			}
			if matchedDir.titleDistance < 0.1 && matchedDir.genreDistance < 0.1 && matchedDir.artistDistance < 0.1 { // ほぼ全一致
				matchLevel = 1000
			} else if matchedDir.titleDistance < 0.1 && matchedDir.artistDistance < 0.1 { // titleほぼ一致 & artistほぼ一致
				matchLevel = 100
			} else if (strings.HasPrefix(dir.Name, entry.title) || strings.HasPrefix(entry.title, pureDirName)) && matchedDir.titleDistance < 0.5 &&
				(strings.HasPrefix(dir.BmsDataSet[0].Genre, entry.genre) || strings.HasPrefix(entry.genre, dir.BmsDataSet[0].Genre)) && matchedDir.genreDistance < 0.8 &&
				(strings.HasPrefix(dir.BmsDataSet[0].Artist, entry.artist) || strings.HasPrefix(entry.artist, dir.BmsDataSet[0].Artist)) && matchedDir.artistDistance < 0.8 { // 全て先頭一致 & 全てあいまい一致
				matchLevel = 7
			} else if (strings.HasPrefix(dir.Name, entry.title) || strings.HasPrefix(entry.title, pureDirName) || matchedDir.titleDistance < 0.5) &&
				(matchedDir.genreDistance < 0.1 && matchedDir.artistDistance < 0.7 ||
					matchedDir.genreDistance < 0.7 && matchedDir.artistDistance < 0.1) { // title先頭一致かあいまい一致 & genre,artist片方ほぼ一致、もう片方あいまい一致
				matchLevel = 6
			} else if matchedDir.titleDistance < 0.1 && matchedDir.genreDistance < 0.1 { // titleほぼ一致 & genreほぼ一致
				matchLevel = 5
			} else if matchedDir.titleDistance < 0.1 && countPrefixMatch(dir.BmsDataSet[0].Artist, entry.artist) >= 3 { // title & artist先頭3文字一致
				matchLevel = 4
			} else if matchedDir.titleDistance < 0.2 && matchedDir.genreDistance < 0.1 { // titleあいまい一致 & genreほぼ一致
				matchLevel = 3
			} else if matchedDir.titleDistance == 0 { // title一致
				matchLevel = 2
			} else if (matchedDir.genreDistance < 0.1 || matchedDir.artistDistance < 0.1) &&
				countPrefixMatch(dir.Name, entry.title) >= 3 &&
				countPrefixMatch(dir.BmsDataSet[0].Genre, entry.genre) >= 3 &&
				countPrefixMatch(dir.BmsDataSet[0].Artist, entry.artist) >= 3 { // genreかartist & 全て先頭3文字一致
				matchLevel = 1
			}

			if matchLevel > 0 && matchLevel > maxMatchLevel {
				matchDirIndex = j
				maxMatchLevel = matchLevel
			}
		}

		if matchDirIndex == -1 {
			unmatchEntries = append(unmatchEntries, unmatchEntry{i, entry})
		} else {
			rankedDirs[i] = unmatchDirs[matchDirIndex]
			if matchDirIndex == len(unmatchDirs)-1 {
				unmatchDirs = unmatchDirs[:matchDirIndex]
			} else {
				unmatchDirs = append(unmatchDirs[:matchDirIndex], unmatchDirs[matchDirIndex+1:]...)
			}
		}
	}

	return rankedDirs, unmatchEntries, unmatchDirs
}

func normalizedLevenshteinDistance(str1, str2 string) float64 {
	distance := lsd.StringDistance(strings.ToLower(str1), strings.ToLower(str2))
	longerTitleLength := math.Max(float64(utf8.RuneCountInString(str1)), float64(utf8.RuneCountInString(str2)))
	normalizedDistance := float64(distance) / longerTitleLength
	return normalizedDistance
}

func countPrefixMatch(str1, str2 string) int {
	i := 0
	for ; i < utf8.RuneCountInString(str1); i++ {
		if string([]rune(str1)[i:i+1]) != string([]rune(str2)[i:i+1]) {
			break
		}
	}
	return i
}

func outputCsv(entries []bmsEntryInfo, rankedDirs []gobms.BmsDirectory, unmatchEntries []unmatchEntry, remainingBmsDirs []gobms.BmsDirectory) error {
	records := [][]string{}

	for i, entry := range entries {
		name := "###Unmatch###"
		if rankedDirs[i].Name != "" {
			name = rankedDirs[i].Name
		}
		records = append(records,
			[]string{fmt.Sprintf("%d", i+1), entry.title, name, rankedDirs[i].Path})
	}

	if len(unmatchEntries) > 0 {
		records = append(records, []string{"---Unmatch bms Entries---", "", "", ""})
		for _, entry := range unmatchEntries {
			records = append(records, []string{fmt.Sprintf("%d", entry.index+1), entry.entryInfo.title, "###Unmatch###", ""})
		}

		if len(remainingBmsDirs) > 0 {
			records = append(records, []string{"---Remaining bms directories---", "", "", ""})
			for _, dir := range remainingBmsDirs {
				records = append(records, []string{"", "", dir.Name, dir.Path})
			}
		}
	}

	csvbuf := new(bytes.Buffer)
	w := csv.NewWriter(csvbuf)
	if err := w.WriteAll(records); err != nil {
		return fmt.Errorf("csv text write: %w", err)
	}

	file, err := os.Create("rankOutput.csv")
	if err != nil {
		return fmt.Errorf("csv file create: %w", err)
	}
	defer file.Close()
	_, err = file.Write(csvbuf.Bytes())
	if err != nil {
		return fmt.Errorf("csv file write: %w", err)
	}
	fmt.Println("Done: rankOutput.csv generated.")

	return nil
}

func loadAllMatchedCsv(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("csv open: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	var line []string
	var paths []string

	for {
		line, err = reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("csv read: %w", err)
		} else {
			paths = append(paths, line[3])
		}
	}

	return paths, nil
}

type Table struct {
	Name   string   `json:"name"`
	Course []Course `json:"course"`
}

type Course struct {
	Class   string      `json:"class"`
	Name    string      `json:"name"`
	Hash    []CourseBms `json:"hash"`
	Trophy  []Trophy    `json:"trophy"`
	Release bool        `json:"release"`
}

type Trophy struct {
	Class     string  `json:"class"`
	Name      string  `json:"name"`
	Missrate  float64 `json:"missrate"`
	Scorerate int     `json:"scorerate"`
}

type CourseBms struct {
	Title   string `json:"title"`
	Genre   string `json:"genre"`
	Artist  string `json:"artist"`
	Md5     string `json:"md5,omitempty"`
	Sha256  string `json:"sha256"`
	Content int    `json:"content"`
}

func outputCource(paths []string) error {
	trophyClass := "bms.player.beatoraja.CourseData$TrophyData"
	bronze := Trophy{trophyClass, "bronzemedal", 7.5, 55}
	silver := Trophy{trophyClass, "silvermedal", 5, 70}
	gold := Trophy{trophyClass, "goldmedal", 2.5, 85}
	trophies := []Trophy{bronze, silver, gold}

	var courses []Course
	var cBmses []CourseBms
	start := len(paths)
	for i := len(paths) - 1; i >= 0; i-- {
		bmsdir, err := gobms.LoadBmsInDirectory(paths[i])
		if err != nil {
			return fmt.Errorf("LoadBmsInDirectory: %w", err)
		}
		/*for _, b := range bmsdir.Bmsfiles {
		  fmt.Printf("%s %s %dKey Dif%s Lv%s\n", b.Title, b.Subtitle, b.Keymode, b.Difficulty, b.Playlevel)
		}*/
		bms := selectBestChartForCourse(bmsdir)
		cBms := CourseBms{Title: bms.Title, Genre: bms.Genre, Artist: bms.Artist,
			Md5: bms.Md5, Sha256: bms.Sha256, Content: 2}
		cBmses = append(cBmses, cBms)

		if (i+1)%50 == 1 {
			course := Course{Class: "bms.player.beatoraja.CourseData",
				Name: fmt.Sprintf("%d~%d", start, i+1), Hash: cBmses,
				Trophy: trophies, Release: false}
			courses = append(courses, course)
			cBmses = []CourseBms{}
			start = i
		}
	}
	tbl := Table{Name: "dcmake", Course: courses}

	f, err := os.Create("./digestCourse.json")
	if err != nil {
		return fmt.Errorf("json create: %w", err)
	}
	defer f.Close()

	// EscapeHTMLをfalseにするためにmarshalではなくencoderを使用
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "\t")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(tbl); err != nil {
		return fmt.Errorf("json encode: %w", err)
	}

	fmt.Println("output digestCourse.json")

	return nil
}

func selectBestChartForCourse(bmsdir gobms.BmsDirectory) gobms.BmsData {
	keyPriority := map[int]int{7: 1, 5: 2, 14: 3, 10: 4, 9: 5, 24: 6, 48: 7}

	var bestBms gobms.BmsData
	for i, bmsfile := range bmsdir.BmsDataSet {
		if i == 0 {
			bestBms = bmsfile
			continue
		}
		if keyPriority[bmsfile.Keymode] < keyPriority[bestBms.Keymode] {
			bestBms = bmsfile
		} else if bmsfile.Keymode == bestBms.Keymode {
			bmsDif, _ := strconv.Atoi(bmsfile.Difficulty)
			bestDif, _ := strconv.Atoi(bestBms.Difficulty)
			if (bmsDif <= 4 && bmsDif > bestDif) || (bmsDif == 4 && bestDif == 5) {
				bestBms = bmsfile
			} else if bmsDif == bestDif { // todo
				maxLevel := 12
				if bmsfile.Keymode == 9 {
					maxLevel = 50
				}
				bmsLevel, _ := strconv.Atoi(bmsfile.Playlevel)
				bestLevel, _ := strconv.Atoi(bestBms.Playlevel)
				if bmsLevel <= maxLevel && bmsLevel > bestLevel {
					bestBms = bmsfile
				}
			}
		}
	}

	return bestBms
}
