## MultiPart file downloader in Golang

> [!CAUTION]
> This project is made for educational purposes, it's not the best code, it's not my best work, it's just here to answer some specific questions

Sample File To Download:
```
https://dts.podtrac.com/redirect.mp3/chrt.fm/track/18987/api.spreaker.com/download/episode/56636674/2070_0905.mp3
```

As an example, you can run:
```
go run main.go download https://dts.podtrac.com/redirect.mp3/chrt.fm/track/18987/api.spreaker.com/download/episode/56636674/2070_0905.mp3
```

And output will be like:
```
downloading podcast url: https://dts.podtrac.com/redirect.mp3/chrt.fm/track/18987/api.spreaker.com/download/episode/56636674/2070_0905.mp3
processing multiple
0 % downloaded
range 728880-1093319 started
range 1457760-1822198 started
range 1093320-1457759 started
range 0-364439 started
range 364440-728879 started
0 % downloaded
0 % downloaded
0 % downloaded
started writing to buffer
0 % downloaded
3 % downloaded
6 % downloaded
14 % downloaded
started writing to buffer
364440 <nil>
25 % downloaded
37 % downloaded
364438 <nil>
started writing to buffer
51 % downloaded
started writing to buffer
started writing to buffer
364440 <nil>
69 % downloaded
86 % downloaded
364440 <nil>
364440 <nil>
file successfully is written to: /Users/arsham/Projects/mine/multidownloader/2070_0905.mp3
```
