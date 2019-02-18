# sliyy
sliyy is CLI tool to output SLI as stdout in csv format.

## Install
```
go get -u github.com/atsushi-ishibashi/sliyy/cmd/sliyy
```

## Usage
```
$ sliyy -h
Usage of ./sliyy:
  -end string
    	end time, default is now, RFC3339 (default "2019-02-18T09:39:53+09:00")
  -period string
    	period, valid formats: 5m,5h,5d (default "1h")
  -sli string
    	SLI type, valid values: availability, latency
  -start string
    	start time, RFC3339
```
