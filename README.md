# dcmake (digest course make)
BMSダイジェスト撮影用のbeatorajaコースファイルを作成します。

- Another譜面を優先してコースに使用します。
- コースは50曲ごとに分割して作成します。

## Usage
```
dcmake {<bms rootdir path> <rank csv path> | -all <all matched csv path>}
```

出力されたコースファイル`digestCourse.json`は、beatorajaの`table`フォルダ配下に配置する。

## Example
- `for_digest_BMS`: ダイジェスト用のBMSを入れたフォルダ
- `digest_rank.csv`: ダイジェスト用のBMSのデータをランク順(昇順)で記述したcsv。データは`Title,Genre,Artist`の順。

digest_rank.csv 
```
Title_1st,Genre_1st,Artist_1st
Title_2nd,Genre_2nd,Artist_2nd
Title_3rd,Genre_3rd,Artist_3rd
...
Title_199th,Genre_199th,Artist_199th
Title_200th,Genre_200th,Artist_200th
```

### 実行(すべてマッチング成功)
```
> dcmake for_digest_BMS digest_rank.csv
match 200, unmatch 0, remaining bms directories 2
output digestCourse.json
```

### 実行(マッチング失敗あり)
```
> dcmake for_digest_BMS digest_rank.csv
match 197, unmatch 3, remaining bms directories 5
Done: rankOutput.csv generated.
```
対応するBMSのディレクトリのマッチングに失敗したデータがある場合、`rankOutput.csv`が出力される。

rankOutput.csv
```
1,Title_1st,chartname_1st,directorypath_1st
2,Title_2nd,chartname_2nd,directorypath_2nd
3,Title_3rd,chartname_3rd,directorypath_3rd
...
199,Title_199th,###Unmatch###,
200,Title_200th,chartname_200th,directorypath_200th
---Unmatch bms Entries---,,,
52,Title_52nd,###Unmatch###,
117,Title_117th,###Unmatch###,
199,Title_199nd,###Unmatch###,
---Remaining bms directories---,,,
,,,directorypath_52th
,,,directorypath_117th
,,,directorypath_199th
,,,directorypath_another1
,,,directorypath_another2
```
rankOutput.csvをテキストエディタで開き、`###Unmatch###`のある行の第4カラムに対応するBMSディレクトリパスを記述する。  

rankOutput_allmatched.csv
```
1,Title_1st,chartname_1st,directorypath_1st
2,Title_2nd,chartname_2nd,directorypath_2nd
3,Title_3rd,chartname_3rd,directorypath_3rd
...
199,Title_199th,###Unmatch###,directorypath_199th
200,Title_200th,chartname_200th,directorypath_200th
```

対応するパスをすべて埋めたら、今度は`-all`オプションを付けて実行する。
```
> dcmake -all rankOutput_allmatched.csv
output digestCourse.json
```