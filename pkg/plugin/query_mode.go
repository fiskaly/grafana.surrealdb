package plugin

import "fmt"

type QueryMode uint8

const (
	_RAW    = "raw"
	_LOG    = "log"
	_METRIC = "metric"
)

const (
	UndefinedQueryMode QueryMode = iota
	RawQueryMode
	LogQueryMode
	MetricQueryMode
)

func NewQueryMode(value string) (QueryMode, error) {
	switch value {
	case _RAW:
		return RawQueryMode, nil
	case _LOG:
		return LogQueryMode, nil
	case _METRIC:
		return MetricQueryMode, nil
	default:
		return UndefinedQueryMode, fmt.Errorf("unsupported query mode '%s'", value)
	}
}

func (r QueryMode) String() string {
	switch r {
	case RawQueryMode:
		return _RAW
	case LogQueryMode:
		return _LOG
	case MetricQueryMode:
		return _METRIC
	default:
		return "" // explicitly requested behavior by Grafana plugin reviewer
	}
}
