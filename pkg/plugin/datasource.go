package plugin

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/surrealdb/surrealdb.go"
)

//go:embed datasource.surql
var datasource_surql []byte

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend
var (
	_ backend.QueryDataHandler    = (*Datasource)(nil)
	_ backend.CheckHealthHandler  = (*Datasource)(nil)
	_ backend.CallResourceHandler = (*Datasource)(nil)
)

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt
var (
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

type dataModel struct {
	Location  string `json:"location"`
	Namespace string `json:"nameaddr"`
	Database  string `json:"database"`
	Username  string `json:"username"`
	Password  string `json:"password"` // secrets
}

type Datasource struct {
	db     *surrealdb.DB
	config dataModel
}

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend#DataSourceInstanceSettings
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var undefined instancemgmt.Instance

	var jsonData dataModel
	err := json.Unmarshal(settings.JSONData, &jsonData)
	if err != nil {
		log.DefaultLogger.Error("JSONData", "Error", err)
		return undefined, err
	}

	config := dataModel{
		Location:  "localhost:8000",
		Namespace: "default",
		Database:  "default",
		Username:  "root",
		Password:  "root",
	}

	if jsonData.Location != "" {
		config.Location = jsonData.Location
	}
	if jsonData.Namespace != "" {
		config.Namespace = jsonData.Namespace
	}
	if jsonData.Database != "" {
		config.Database = jsonData.Database
	}
	if jsonData.Username != "" {
		config.Username = jsonData.Username
	}

	var secureData = settings.DecryptedSecureJSONData
	if secureData != nil {
		if secureData["password"] != "" {
			config.Password = secureData["password"]
		}
	}

	location := fmt.Sprintf("ws://%s/rpc", config.Location)
	db, err := surrealdb.New(location)
	if err != nil {
		return undefined, err
	}

	if _, err = db.Signin(map[string]interface{}{
		"user": config.Username,
		"pass": config.Password,
	}); err != nil {
		return undefined, err
	}

	if _, err = db.Use(
		config.Namespace,
		config.Database,
	); err != nil {
		return undefined, err
	}

	r := &Datasource{
		db:     db,
		config: config,
	}

	// pre-load the 'fn::grafana::rate' function
	_, err = r.query(string(datasource_surql))
	if err != nil {
		return undefined, err
	}

	return r, nil
}

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt#InstanceDisposer
func (r *Datasource) Dispose() {
	r.db.Close()
}

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend#CheckHealthHandler
func (r *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	var status = backend.HealthStatusOk
	var message = "Data source is working"

	query := `
return
{ database : session::db()
, namespace : session::ns()
, origin : session::origin()
}
`
	_, err := r.query(query)
	if err != nil {
		status = backend.HealthStatusError
		message = "Data source unhealthy: " + err.Error()
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend#QueryDataHandler
func (r *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()

	for _, q := range req.Queries {
		res := r.queryData(ctx, req.PluginContext, q)
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type queryModel struct {
	Mode          string   `json:"mode"`
	SurQL         string   `json:"surql"`
	Requery       bool     `json:"requery"`
	Timestamp     string   `json:"timestamp"`
	LogMessage    string   `json:"logMessage"`
	MetricData    string   `json:"metricData"`
	Rate          bool     `json:"rate"`
	RateZero      bool     `json:"rateZero"`
	RateInterval  string   `json:"rateInterval"`
	RateFunctions []string `json:"rateFunctions"`
}

type queryResponseData struct {
	status interface{}
	time   interface{}
	result interface{}
}

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
		panic(fmt.Sprintf("invalid QueryMode state '%d'", r))
	}
}

func (r *Datasource) queryData(ctx context.Context, pCtx backend.PluginContext, dataQuery backend.DataQuery) backend.DataResponse {

	queryTimeNow := time.Now().UTC()

	queryTimeFrom := dataQuery.TimeRange.From
	queryTimeFrom = queryTimeFrom.Truncate(time.Second)
	queryTimeFrom = queryTimeFrom.Add(time.Nanosecond * -1)

	queryTimeTo := dataQuery.TimeRange.To
	queryTimeTo = queryTimeTo.Truncate(time.Second)
	queryTimeTo = queryTimeTo.Add(time.Second * 1)
	queryTimeTo = queryTimeTo.Add(time.Nanosecond * -1)

	queryName := dataQuery.RefID
	queryInterval := dataQuery.Interval

	// log.DefaultLogger.Info(
	// 	fmt.Sprintf("query: %s", dataQuery.JSON),
	// )

	var query queryModel

	err := json.Unmarshal(dataQuery.JSON, &query)
	if err != nil {
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Query json: %v", err.Error()),
		)
	}

	queryMode, err := NewQueryMode(query.Mode)
	if err != nil {
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Query mode: %v", err.Error()),
		)
	}

	// log.DefaultLogger.Info(
	// 	fmt.Sprintf(
	// 		"query: %v, from: %s, to: %s, type: %s, refId: %s, mdp: %s, int: %s, mode: '%s'",
	// 		query,
	// 		queryTimeFrom.Format(time.RFC3339Nano),
	// 		queryTimeTo.Format(time.RFC3339Nano),
	// 		dataQuery.QueryType,     // ?
	// 		dataQuery.RefID,         // string
	// 		dataQuery.MaxDataPoints, // int64
	// 		dataQuery.Interval,      // time.Time
	// 		queryMode.String(),
	// 	),
	// )

	surql := query.SurQL

	if strings.HasPrefix(surql, "select") == false &&
		strings.HasPrefix(surql, "SELECT") == false &&
		strings.HasPrefix(surql, "info") == false &&
		strings.HasPrefix(surql, "INFO") == false &&
		strings.HasPrefix(surql, "return") == false &&
		strings.HasPrefix(surql, "RETURN") == false {
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Query `%s` is not allowed", surql),
		)
	}

	if query.Timestamp == "" {
		query.Timestamp = "timestamp"
	}

	if query.MetricData == "" {
		query.MetricData = "value"
	}

	if queryMode == MetricQueryMode && query.Rate {

		rateFunctions := []string{"absence", "average", "sum", "median", "stddev", "quantile25", "quantile75", "quantile95", "quantile99"}
		quantiles := []string{}

		options := fmt.Sprintf(
			"{ time : { key : %q }, value : { key : %q",
			query.Timestamp,
			query.MetricData,
		)

		if query.RateZero {
			options = options + ", zero : true"
		}

		for _, rateFunction := range rateFunctions {
			if slices.Contains(query.RateFunctions, rateFunction) {
				if strings.HasPrefix(rateFunction, "quantile") {
					quantile := rateFunction[8:len(rateFunction)]
					quantiles = append(quantiles, quantile)
				} else {
					options = options + ", " + rateFunction + " : true"
				}
			}
		}

		if len(quantiles) > 0 {
			options = options + ", quantile : [ "
			for _, quantile := range quantiles {
				options = options + quantile + ", "
			}
			options = options[:len(options)-2] + "]"
		}

		options = options + "} }"

		queryRateInterval := query.RateInterval
		if queryRateInterval == "" {
			queryRateInterval = "$interval"
		}

		surql = fmt.Sprintf(
			"fn::rate( %s, $from, $to, %s, %s )",
			queryRateInterval,
			options,
			surql,
		)
	}

	surql = strings.Replace(surql, "$interval", queryInterval.String(), -1)
	surql = strings.Replace(surql, "$now", "'"+queryTimeNow.Format(time.RFC3339Nano)+"'", -1)
	surql = strings.Replace(surql, "$from", "'"+queryTimeFrom.Format(time.RFC3339Nano)+"'", -1)
	surql = strings.Replace(surql, "$to", "'"+queryTimeTo.Format(time.RFC3339Nano)+"'", -1)

	queryResponse, err := r.query(surql)
	if err != nil {
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Query failed: %v", err.Error()),
		)
	}

	var preferredVisualization data.VisType
	switch queryMode {
	case RawQueryMode:
		preferredVisualization = data.VisTypeTable
	case LogQueryMode:
		preferredVisualization = data.VisTypeLogs
	case MetricQueryMode:
		preferredVisualization = data.VisTypeGraph
	default:
		// TODO: @ppaulweber: provide more modes in the future
		// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/data#pkg-constants
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Type '%s' not supported", queryMode),
		)
	}

	// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/data#FrameMeta
	frameMeta := &data.FrameMeta{
		PreferredVisualization: preferredVisualization,
		Custom: struct {
			QueryRaw string `json:"queryRaw"`
			QueryRun string `json:"queryRun"`
			Status   string `json:"status"`
			Time     string `json:"time"`
			// Result   string `json:"result"`
			// Request  string `json:"request"`
		}{
			QueryRaw: fmt.Sprintf("%s", query.SurQL),
			QueryRun: fmt.Sprintf("%s", surql),
			Status:   fmt.Sprintf("%s", queryResponse.status),
			Time:     fmt.Sprintf("%s", queryResponse.time),
			// Result:   fmt.Sprintf("%s", queryResponse.result),
			// Request:  fmt.Sprintf("%s", dataQuery.JSON),
		},
	}

	var dataResponse backend.DataResponse

	err = r.process(&query, queryName, queryResponse.result, frameMeta, &dataResponse)
	if err != nil {
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Result failed: %v", err.Error()),
		)
	}

	return dataResponse
}

func (r *Datasource) process(query *queryModel, name string, result interface{}, frameMeta *data.FrameMeta, dataResponse *backend.DataResponse) error {

	if result == nil {
		frame, err := r.result(query, name, "null")
		if err != nil {
			return err
		}

		frame.Meta = frameMeta
		dataResponse.Frames = append(dataResponse.Frames, frame)
		return nil
	}

	if value, isValue := result.(string); isValue {
		frame, err := r.result(query, name, value)
		if err != nil {
			return err
		}

		frame.Meta = frameMeta
		dataResponse.Frames = append(dataResponse.Frames, frame)
		return nil
	}

	if value, isValue := result.(float64); isValue {
		frame, err := r.result(query, name, value)
		if err != nil {
			return err
		}

		frame.Meta = frameMeta
		dataResponse.Frames = append(dataResponse.Frames, frame)
		return nil
	}

	if table, isTable := result.([]interface{}); isTable {
		frame, err := r.table(query, name, table)
		if err != nil {
			return err
		}

		frame.Meta = frameMeta
		dataResponse.Frames = append(dataResponse.Frames, frame)
		return nil
	}

	if tables, isTables := result.(map[string]interface{}); isTables {
		for tableName, tableResult := range tables {
			err := r.process(
				query,
				fmt.Sprintf("%s:%s", name, tableName),
				tableResult,
				frameMeta,
				dataResponse,
			)
			if err != nil {
				return err
			}
		}
		return nil
	}

	return fmt.Errorf("not supported query result type '%s'", reflect.TypeOf(result))
}

func (r *Datasource) result(query *queryModel, name string, value interface{}) (*data.Frame, error) {
	// var undefined *data.Frame

	head := make(map[string]int)
	table := make([]map[string]interface{}, 0)

	head["result"] = 1
	table = append(table, map[string]interface{}{"result": value})

	return r.frame(query, name, head, table)
}

func (r *Datasource) table(query *queryModel, name string, array []interface{}) (*data.Frame, error) {
	// var undefined *data.Frame

	head := make(map[string]int)
	table := make([]map[string]interface{}, 0)

	for _, entry := range array {
		// log.DefaultLogger.Info(fmt.Sprintf("entry: %s", entry))

		if entryMap, isEntryMap := entry.(map[string]interface{}); isEntryMap {
			row := make(map[string]interface{})
			for key, value := range entryMap {

				if headKey, headKeyExists := head[key]; headKeyExists {
					head[key] = headKey + 1
				} else {
					head[key] = 0
				}

				row[key] = value
				// log.DefaultLogger.Info(fmt.Sprintf("key->value: %s->%s", key, value))
			}

			table = append(table, row)
		}
	}

	return r.frame(query, name, head, table)
}

// https://grafana.com/developers/plugin-tools/introduction/data-frames
// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/data#Frame
func (r *Datasource) frame(query *queryModel, name string, head map[string]int, table []map[string]interface{}) (*data.Frame, error) {
	// var undefined *data.Frame

	frame := data.NewFrame(name)

	columns := make(map[string][]string)
	for _, row := range table {
		for key, _ := range head {

			column, columnExists := columns[key]
			if columnExists == false {
				column = make([]string, 0)
			}

			value := ""

			cell, cellExists := row[key]
			if cellExists {
				cellBytes, err := json.Marshal(cell)
				if err != nil {
					value = fmt.Sprintf("< %s >", cell)
				} else {
					value = string(cellBytes)
					if strings.HasPrefix(value, "\"") &&
						strings.HasSuffix(value, "\"") {
						value = value[1 : len(value)-1]
					}
				}
			}
			column = append(column, value)

			columns[key] = column
		}
	}

	fields := make(map[string]interface{})
	keys := make([]string, 0)

	timestampKey := query.Timestamp
	timestampColumn, timestampExists := columns[timestampKey]
	if timestampExists {
		timestamps := make([]*time.Time, len(timestampColumn))

		for tsIndex, tsValue := range timestampColumn {
			// https://pkg.go.dev/time#pkg-constants
			timestamp, err := time.Parse(time.RFC3339Nano, tsValue)

			if err != nil {
				timestamps[tsIndex] = nil
				log.DefaultLogger.Error(err.Error())
			} else {
				timestamps[tsIndex] = &timestamp
			}
		}

		delete(columns, timestampKey)
		fields[timestampKey] = timestamps
	}

	for key, column := range columns {
		fields[key] = column
		if key == "id" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if _, hasId := fields["id"]; hasId {
		keys = append([]string{"id"}, keys...)
	}

	if _, hasTimestamp := fields[timestampKey]; hasTimestamp {
		keys = append([]string{timestampKey}, keys...)
	}

	for _, key := range keys {
		field := fields[key]

		field = r.typeConversion(field)

		dataField := data.NewField(key, nil, field)

		frame.Fields = append(
			frame.Fields,
			dataField,
		)
	}

	return frame, nil
}

func (r *Datasource) typeConversion(field interface{}) interface{} {
	fieldArray, isFieldArray := field.([]string)
	if isFieldArray == false || len(fieldArray) == 0 {
		return field
	}

	result := make([]*float64, 0)
	for _, fieldValue := range fieldArray {
		if fieldValue == "null" {
			result = append(result, nil)
			continue
		}

		value, err := strconv.ParseFloat(fieldValue, 64)
		if err != nil {
			return field
		}

		result = append(result, &value)
	}

	return result
}

func (r *Datasource) CallResource(ctx context.Context, request *backend.CallResourceRequest, response backend.CallResourceResponseSender) error {
	// log.DefaultLogger.Info(fmt.Sprintf("CallResource: %s", request.Path))

	switch request.Path {
	// case "TODO":
	// 	return response.Send(
	// 		&backend.CallResourceResponse{
	// 			Status: http.StatusOK,
	// 			Body:   []byte(`{}`),
	// 		},
	// 	)
	default:
		return response.Send(
			&backend.CallResourceResponse{
				Status: http.StatusNotFound,
				Body:   []byte(`{}`),
			},
		)
	}
}

func (r *Datasource) query(query string) (queryResponseData, error) {
	var undefined queryResponseData

	queryResponse, err := r.db.Query(query, map[string]interface{}{})
	if err != nil {
		return undefined, err
	}

	// log.DefaultLogger.Info(fmt.Sprintf("queryResponse: %s", queryResponse))

	array, isArray := queryResponse.([]interface{})
	if len(array) == 0 {
		return undefined, fmt.Errorf("invalid queryResponse length")
	}

	// log.DefaultLogger.Info(fmt.Sprintf("arrResp: %s", arrResp))

	dataMap, ok := array[0].(map[string]interface{})
	if ok == false || isArray == false {
		return undefined, fmt.Errorf("invalid queryResponse array type")
	}

	// log.DefaultLogger.Info(fmt.Sprintf("dataMap: %s", dataMap))

	status, ok := dataMap["status"]
	if ok == false {
		return undefined, fmt.Errorf("invalid queryResponse status")
	}

	// log.DefaultLogger.Info(fmt.Sprintf("status: %s", status))

	responseTime, ok := dataMap["time"]
	if ok == false {
		return undefined, fmt.Errorf("invalid queryResponse time")
	}

	// log.DefaultLogger.Info(fmt.Sprintf("time: %s", responseTime))

	result, ok := dataMap["result"]
	if ok == false {
		return undefined, fmt.Errorf("invalid queryResponse data")
	}

	return queryResponseData{
		status: status,
		time:   responseTime,
		result: result,
	}, nil
}
