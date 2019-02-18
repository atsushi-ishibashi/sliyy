package svc

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/atsushi-ishibashi/sliyy/model"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
)

type ListMetricsInput struct {
	NameSpace  string
	Metrics    []string
	Dimentions []string
}

func (i ListMetricsInput) validate() error {
	if i.NameSpace == "" {
		return errors.New("ListMetricsInput.NameSpace is empty")
	}
	if len(i.Dimentions) == 0 {
		return errors.New("ListMetricsInput.Dimentions is empty")
	}
	return nil
}

func (i ListMetricsInput) matchDimentions(dims []*cloudwatch.Dimension) bool {
	if len(i.Dimentions) != len(dims) {
		return false
	}
	matchCount := 0
	for _, v := range dims {
		for _, vv := range i.Dimentions {
			if vv == *v.Name {
				matchCount++
				break
			}
		}
	}
	return len(i.Dimentions) == matchCount
}

type GetMetricStatisticInput struct {
	NameSpace  string
	MetricName string
	Dimensions []model.MetricDimension
	Period     time.Duration
	Start      time.Time
	End        time.Time
	Type       StaticType
}

type StaticType string

const (
	StaticAverage     StaticType = "Average"
	StaticSampleCount StaticType = "SampleCount"
	StaticSum         StaticType = "Sum"
	StaticMinimum     StaticType = "Minimum"
	StaticMaximum     StaticType = "Maximum"
)

func (st StaticType) validate() error {
	switch st {
	case StaticAverage, StaticSampleCount, StaticSum,
		StaticMinimum, StaticMaximum:
		return nil
	default:
		return fmt.Errorf("invalid StaticType %s", st)
	}
}

func (i GetMetricStatisticInput) validate() error {
	if i.NameSpace == "" {
		return errors.New("GetMetricStatisticInput.NameSpace is empty")
	}
	if i.MetricName == "" {
		return errors.New("GetMetricStatisticInput.MetricName is empty")
	}
	if len(i.Dimensions) == 0 {
		return errors.New("GetMetricStatisticInput.Dimensions is empty")
	}
	if i.Period == 0 {
		return errors.New("GetMetricStatisticInput.Period is empty")
	}
	if i.Start.IsZero() {
		return errors.New("GetMetricStatisticInput.Start is empty")
	}
	if i.End.IsZero() {
		return errors.New("GetMetricStatisticInput.End is empty")
	}
	if err := i.Type.validate(); err != nil {
		return err
	}
	return nil
}

type MetricsService interface {
	ListMetrics(input ListMetricsInput) ([]model.Metric, error)
	GetMetricStatistic(input GetMetricStatisticInput) ([]model.MetricStatistic, error)
}

type cwmService struct {
	svc cloudwatchiface.CloudWatchAPI
}

func NewMetricsService() MetricsService {
	return &cwmService{
		svc: cloudwatch.New(session.New(), aws.NewConfig().WithRegion(os.Getenv("_SLIYY_AWS_REGION"))),
	}
}

func (s *cwmService) ListMetrics(input ListMetricsInput) ([]model.Metric, error) {
	if err := input.validate(); err != nil {
		return nil, err
	}
	if len(input.Metrics) > 0 {
		return s.listMetricsFilterName(input)
	}
	result := make([]model.Metric, 0)
	param := &cloudwatch.ListMetricsInput{
		Namespace: aws.String(input.NameSpace),
	}
	err := s.svc.ListMetricsPages(param,
		func(page *cloudwatch.ListMetricsOutput, lastPage bool) bool {
			for _, met := range page.Metrics {
				if input.matchDimentions(met.Dimensions) {
					dims := make([]model.MetricDimension, 0)
					for _, dim := range met.Dimensions {
						dims = append(dims, model.MetricDimension{
							Name:  *dim.Name,
							Value: *dim.Value,
						})
					}
					result = append(result, model.Metric{
						Namespace:  input.NameSpace,
						Name:       *met.MetricName,
						Dimensions: dims,
					})
				}
			}
			return page.NextToken != nil
		})
	if err != nil {
		return nil, nil
	}
	return result, nil
}

func (s *cwmService) listMetricsFilterName(input ListMetricsInput) ([]model.Metric, error) {
	result := make([]model.Metric, 0)
	for _, metName := range input.Metrics {
		param := &cloudwatch.ListMetricsInput{
			Namespace:  aws.String(input.NameSpace),
			MetricName: aws.String(metName),
		}
		err := s.svc.ListMetricsPages(param,
			func(page *cloudwatch.ListMetricsOutput, lastPage bool) bool {
				for _, met := range page.Metrics {
					if input.matchDimentions(met.Dimensions) {
						dims := make([]model.MetricDimension, 0)
						for _, dim := range met.Dimensions {
							dims = append(dims, model.MetricDimension{
								Name:  *dim.Name,
								Value: *dim.Value,
							})
						}
						result = append(result, model.Metric{
							Namespace:  input.NameSpace,
							Name:       *met.MetricName,
							Dimensions: dims,
						})
					}
				}
				return page.NextToken != nil
			})
		if err != nil {
			return nil, nil
		}
	}
	return result, nil
}

func (s *cwmService) GetMetricStatistic(input GetMetricStatisticInput) ([]model.MetricStatistic, error) {
	if err := input.validate(); err != nil {
		return nil, err
	}
	result := make([]model.MetricStatistic, 0)
	splitTimes := splitTime(input.Start, input.End, input.Period)
	for _, times := range splitTimes {
		param := &cloudwatch.GetMetricStatisticsInput{
			Namespace:  aws.String(input.NameSpace),
			MetricName: aws.String(input.MetricName),
			StartTime:  aws.Time(times[0]),
			EndTime:    aws.Time(times[1]),
			Period:     aws.Int64(int64(input.Period.Seconds())),
			Statistics: []*string{aws.String(string(input.Type))},
			Dimensions: make([]*cloudwatch.Dimension, 0),
		}
		for _, dim := range input.Dimensions {
			param.Dimensions = append(param.Dimensions, &cloudwatch.Dimension{
				Name: aws.String(dim.Name), Value: aws.String(dim.Value),
			})
		}
		resp, err := s.svc.GetMetricStatistics(param)
		if err != nil {
			return result, err
		}
		for _, v := range resp.Datapoints {
			ms := model.MetricStatistic{
				Timestamp: aws.TimeValue(v.Timestamp),
			}
			switch input.Type {
			case StaticSum:
				ms.Value = aws.Float64Value(v.Sum)
			case StaticAverage:
				ms.Value = aws.Float64Value(v.Average)
			case StaticMaximum:
				ms.Value = aws.Float64Value(v.Maximum)
			case StaticMinimum:
				ms.Value = aws.Float64Value(v.Minimum)
			case StaticSampleCount:
				ms.Value = aws.Float64Value(v.SampleCount)
			}
			result = append(result, ms)
		}
	}
	sort.Sort(model.MetricStatisticList(result))
	return result, nil
}

func splitTime(start, end time.Time, period time.Duration) [][2]time.Time {
	limit := 1441 //depends on cloudwatch specification
	diff := end.Sub(start)
	unit := int(diff / period)
	mod := unit/limit + 1
	result := make([][2]time.Time, 0, mod)
	count := 0
	for count < mod {
		ts := start.Add(period * time.Duration(limit) * time.Duration(count))
		te := start.Add(period * time.Duration(limit-1) * time.Duration(count+1))
		if end.Before(te) {
			te = end
		}
		result = append(result, [2]time.Time{ts, te})
		count++
	}
	return result
}
