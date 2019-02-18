package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/atsushi-ishibashi/sliyy/model"
	"github.com/atsushi-ishibashi/sliyy/svc"
)

var (
	start   = flag.String("start", "", "start time, RFC3339")
	end     = flag.String("end", time.Now().Format(time.RFC3339), "end time, default is now, RFC3339")
	sliType = flag.String("sli", "", "SLI type, valid values: availability, latency")
	period  = flag.String("period", "1h", "period, valid formats: 5m,5h,5d")

	startTime, endTime time.Time

	periodDuration time.Duration

	cwmSvc svc.MetricsService
)

func main() {
	flag.Parse()
	if t, err := time.Parse(time.RFC3339, *start); err != nil {
		log.Fatalln("-start: ", err)
	} else {
		startTime = t
	}
	if t, err := time.Parse(time.RFC3339, *end); err != nil {
		log.Fatalln("-end: ", err)
	} else {
		endTime = t
	}

	if err := SLIType(*sliType).validate(); err != nil {
		log.Fatalln(err)
	}

	if dur, err := parsePeriod(*period); err != nil {
		log.Fatalln(err)
	} else {
		periodDuration = dur
	}

	region := os.Getenv("AWS_REGION")
	defaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	if region != "" {
		os.Setenv("_SLIYY_AWS_REGION", region)
	} else if defaultRegion != "" {
		os.Setenv("_SLIYY_AWS_REGION", defaultRegion)
	} else {
		log.Fatalln("env AWS_REGION or AWS_DEFAULT_REGION required")
	}

	cwmSvc = svc.NewMetricsService()

	switch SLIType(*sliType) {
	case SLIAvalability:
		calcAvalability()
	case SLILatency:
		calcLatency()
	}
}

func calcAvalability() {
	metrics, err := cwmSvc.ListMetrics(svc.ListMetricsInput{
		NameSpace:  "AWS/ApplicationELB",
		Metrics:    []string{"RequestCountPerTarget"},
		Dimentions: []string{"TargetGroup"},
	})
	if err != nil {
		log.Fatalln(err)
	}
	met500, err := cwmSvc.ListMetrics(svc.ListMetricsInput{
		NameSpace:  "AWS/ApplicationELB",
		Metrics:    []string{"HTTPCode_Target_5XX_Count"},
		Dimentions: []string{"TargetGroup", "LoadBalancer"},
	})
	if err != nil {
		log.Fatalln(err)
	}
	type Data struct {
		Name                string
		RequestCountStat    []model.MetricStatistic
		Request5xxCountStat []model.MetricStatistic
	}
	md := make(map[string]*Data)
	for _, met := range metrics {
		stats, err := cwmSvc.GetMetricStatistic(svc.GetMetricStatisticInput{
			NameSpace:  met.Namespace,
			MetricName: met.Name,
			Dimensions: met.Dimensions,
			Period:     periodDuration,
			Start:      startTime,
			End:        endTime,
			Type:       svc.StaticSum,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		for _, v := range met.Dimensions {
			if v.Name == "TargetGroup" {
				md[v.Value] = &Data{
					Name: v.Value, RequestCountStat: stats,
				}
				break
			}
		}
	}
	for _, met := range met500 {
		stats, err := cwmSvc.GetMetricStatistic(svc.GetMetricStatisticInput{
			NameSpace:  met.Namespace,
			MetricName: met.Name,
			Dimensions: met.Dimensions,
			Period:     time.Minute * 30,
			Start:      startTime,
			End:        endTime,
			Type:       svc.StaticSum,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		for _, v := range met.Dimensions {
			if v.Name == "TargetGroup" {
				if d, ok := md[v.Value]; ok {
					d.Request5xxCountStat = stats
				} else {
					md[v.Value] = &Data{
						Name: v.Value, Request5xxCountStat: stats,
					}
				}
				break
			}
		}

	}

	writer := os.Stdout
	for k, v := range md {
		writer.WriteString(k + "\n")
		var normCounts, errCounts, avails, timeStampStr []string
		for _, st := range v.RequestCountStat {
			var errCount int
			for _, est := range v.Request5xxCountStat {
				if est.Timestamp.Equal(st.Timestamp) {
					errCount = int(est.Value)
				} else if est.Timestamp.After(st.Timestamp) {
					break
				}
			}
			timeStampStr = append(timeStampStr, st.Timestamp.Format(time.RFC3339))
			normCounts = append(normCounts, fmt.Sprintf("%d", int(st.Value)))
			errCounts = append(errCounts, fmt.Sprintf("%d", errCount))
			var tmpAvailStr string
			if st.Value == 0 {
				tmpAvailStr = "1.0"
			} else {
				tmpAvailStr = fmt.Sprintf("%.3f", (st.Value-float64(errCount))/st.Value)
			}
			avails = append(avails, tmpAvailStr)
		}
		writer.WriteString(strings.Join(timeStampStr, ",") + "\n")
		writer.WriteString(strings.Join(normCounts, ",") + "\n")
		writer.WriteString(strings.Join(errCounts, ",") + "\n")
		writer.WriteString(strings.Join(avails, ",") + "\n")
		writer.WriteString("\n\n")
	}
}

func calcLatency() {
	metrics, err := cwmSvc.ListMetrics(svc.ListMetricsInput{
		NameSpace:  "AWS/ApplicationELB",
		Metrics:    []string{"TargetResponseTime"},
		Dimentions: []string{"TargetGroup", "LoadBalancer"},
	})
	if err != nil {
		log.Fatalln(err)
	}

	writer := os.Stdout
	for _, met := range metrics {
		stats, err := cwmSvc.GetMetricStatistic(svc.GetMetricStatisticInput{
			NameSpace:  met.Namespace,
			MetricName: met.Name,
			Dimensions: met.Dimensions,
			Period:     periodDuration,
			Start:      startTime,
			End:        endTime,
			Type:       svc.StaticAverage,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		for _, v := range met.Dimensions {
			if v.Name == "TargetGroup" {
				writer.WriteString(v.Value + "\n")
				break
			}
		}

		var timeStampStr, latencyStr []string
		for _, st := range stats {
			timeStampStr = append(timeStampStr, st.Timestamp.Format(time.RFC3339))
			latencyStr = append(latencyStr, fmt.Sprintf("%.3f", st.Value))
		}
		writer.WriteString(strings.Join(timeStampStr, ",") + "\n")
		writer.WriteString(strings.Join(latencyStr, ",") + "\n")
		writer.WriteString("\n\n")
	}
}

type SLIType string

const (
	SLIAvalability SLIType = "availability"
	SLILatency     SLIType = "latency"
)

func (s SLIType) validate() error {
	switch s {
	case SLIAvalability, SLILatency:
		return nil
	default:
		return errors.New("invalid -sli")
	}
}

func parsePeriod(s string) (time.Duration, error) {
	rex := regexp.MustCompile(`^\d+(m|h|d)$`)
	if !rex.MatchString(s) {
		return time.Second, errors.New("invalid period format")
	}
	mul, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
	if err != nil {
		return time.Second, err
	}
	if strings.HasSuffix(s, "m") {
		return time.Minute * time.Duration(mul), nil
	} else if strings.HasSuffix(s, "h") {
		return time.Hour * time.Duration(mul), nil
	}
	return time.Hour * 24 * time.Duration(mul), nil
}
