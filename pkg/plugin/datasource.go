package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
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

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend
var (
	_ backend.QueryDataHandler   = (*Datasource)(nil)
	_ backend.CheckHealthHandler = (*Datasource)(nil)
)

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt
var (
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

type datasourceOptions struct {
	Location  string `json:"location"`
	Namespace string `json:"nameaddr"`
	Database  string `json:"database"`
	Scope     string `json:"scope"`
	Username  string `json:"username"`
}

type Datasource struct {
	db     *surrealdb.DB
	config configuration
}

type configuration struct {
	Location  string
	Namespace string
	Database  string
	Scope     string
	Username  string
	Password  string
}

// https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go/backend#DataSourceInstanceSettings
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var undefined instancemgmt.Instance

	var jsonData datasourceOptions
	err := json.Unmarshal(settings.JSONData, &jsonData)
	if err != nil {
		log.DefaultLogger.Error("JSONData", "Error", err)
		return undefined, err
	}

	config := configuration{
		Location:  "localhost:8000",
		Namespace: "default",
		Database:  "default",
		Scope:     "",
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
	if jsonData.Scope != "" {
		config.Scope = jsonData.Scope
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

	// https://docs.surrealdb.com/docs/integration/websocket/#signin
	signinParameters := map[string]interface{}{
		"NS":   config.Namespace,
		"DB":   config.Database,
		"user": config.Username,
		"pass": config.Password,
	}
	if config.Scope != "" {
		signinParameters["SC"] = config.Scope
	}

	_, err = db.Signin(signinParameters)
	if err != nil {
		return undefined, err
	}

	r := &Datasource{
		db:     db,
		config: config,
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

type queryRequestData struct {
	Hide          bool     `json:"hide"` // inherited
	Mode          string   `json:"mode"`
	SurQL         string   `json:"surql"`
	Requery       bool     `json:"requery"`
	Timestamp     string   `json:"timestamp"`
	LogMessage    string   `json:"logMessage"`
	MetricData    string   `json:"metricData"`
	Group         bool     `json:"group"`
	GroupBy       string   `json:"groupBy"`
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

type queryData struct {
	request  queryRequestData
	response queryResponseData
	timeNow  time.Time
	timeFrom time.Time
	timeTo   time.Time
	interval time.Duration
	name     string
	mode     QueryMode
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

	var queryRequest queryRequestData

	err := json.Unmarshal(dataQuery.JSON, &queryRequest)
	if err != nil {
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Query json: %v", err.Error()),
		)
	}

	if queryRequest.Hide {
		return backend.DataResponse{}
	}

	queryMode, err := NewQueryMode(queryRequest.Mode)
	if err != nil {
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Query mode: %v", err.Error()),
		)
	}

	surql := queryRequest.SurQL

	if queryRequest.Timestamp == "" {
		queryRequest.Timestamp = "timestamp"
	}

	if queryRequest.MetricData == "" {
		queryRequest.MetricData = "value"
	}

	if queryRequest.GroupBy == "" {
		queryRequest.GroupBy = "group"
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

	query := queryData{
		request:  queryRequest,
		response: queryResponse,
		timeNow:  queryTimeNow,
		timeFrom: queryTimeFrom,
		timeTo:   queryTimeTo,
		interval: queryInterval,
		name:     queryName,
		mode:     queryMode,
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
			QueryRaw: fmt.Sprintf("%s", queryRequest.SurQL),
			QueryRun: fmt.Sprintf("%s", surql),
			Status:   fmt.Sprintf("%s", queryResponse.status),
			Time:     fmt.Sprintf("%s", queryResponse.time),
			// Result:   fmt.Sprintf("%s", queryResponse.result),
			// Request:  fmt.Sprintf("%s", dataQuery.JSON),
		},
	}

	var dataResponse backend.DataResponse

	err = r.process(&query, query.name, query.response.result, frameMeta, &dataResponse)
	if err != nil {
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("Result failed: %v", err.Error()),
		)
	}

	if queryMode == MetricQueryMode {
		err = r.metric(&query, &dataResponse)
		if err != nil {
			return backend.ErrDataResponse(
				backend.StatusBadRequest,
				fmt.Sprintf("Metric failed: %v", err.Error()),
			)
		}

		if query.request.Group {
			err = r.metricGroup(&query, &dataResponse)
			if err != nil {
				return backend.ErrDataResponse(
					backend.StatusBadRequest,
					fmt.Sprintf("Group failed: %v", err.Error()),
				)
			}
		}

		if query.request.Rate {
			for _, frame := range dataResponse.Frames {
				err = r.metricRate(&query, frame)
				if err != nil {
					return backend.ErrDataResponse(
						backend.StatusBadRequest,
						fmt.Sprintf("Rate failed: %v", err.Error()),
					)
				}
			}
		}
	}

	return dataResponse
}

func (r *Datasource) process(query *queryData, name string, result interface{}, frameMeta *data.FrameMeta, dataResponse *backend.DataResponse) error {

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

func (r *Datasource) result(query *queryData, name string, value interface{}) (*data.Frame, error) {
	head := make(map[string]int)
	table := make([]map[string]interface{}, 0)

	head["result"] = 1
	table = append(table, map[string]interface{}{"result": value})

	return r.frame(query, name, head, table)
}

func (r *Datasource) table(query *queryData, name string, array []interface{}) (*data.Frame, error) {
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
func (r *Datasource) frame(query *queryData, name string, head map[string]int, table []map[string]interface{}) (*data.Frame, error) {
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

	timestampKey := query.request.Timestamp
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

	dataMap, ok := array[len(array)-1].(map[string]interface{})
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

func (r *Datasource) metric(query *queryData, dataResponse *backend.DataResponse) error {
	frames := dataResponse.Frames
	if len(frames) != 1 {
		return fmt.Errorf("multiple frames are not supported yet")
	}

	var timeField *data.Field
	var dataField *data.Field
	var groupByField *data.Field

	timeFieldName := query.request.Timestamp
	dataFieldName := query.request.MetricData
	groupByFieldName := query.request.GroupBy
	suggestions := ""

	frame := frames[0]
	for _, field := range frame.Fields {
		fieldName := field.Name
		if suggestions == "" {
			suggestions = fieldName
		} else {
			suggestions = suggestions + ", " + fieldName
		}

		if fieldName == timeFieldName {
			timeField = field
			continue
		}
		if fieldName == dataFieldName {
			dataField = field
			continue
		}
		if fieldName == groupByFieldName {
			groupByField = field
			continue
		}
	}

	if len(frame.Fields) == 0 {
		timeField = data.NewField(timeFieldName, nil, []*time.Time{})
		dataField = data.NewField(dataFieldName, nil, []*float64{})
	}

	if timeField == nil {
		return fmt.Errorf(
			"time field '%s' not found in data frame, available are: %v",
			timeFieldName,
			suggestions,
		)
	}
	if dataField == nil {
		return fmt.Errorf(
			"data field '%s' not found in data frame, available are: %v",
			dataFieldName,
			suggestions,
		)
	}
	if groupByField == nil && query.request.Group {
		return fmt.Errorf(
			"group by '%s' not found in data frame, available are: %v",
			groupByFieldName,
			suggestions,
		)
	}

	frame.Fields = []*data.Field{timeField, dataField}

	if query.request.Group {
		frame.Fields = append(frame.Fields, groupByField)
	}

	return nil
}

func (r *Datasource) metricGroup(query *queryData, dataResponse *backend.DataResponse) error {
	frames := dataResponse.Frames
	if len(frames) != 1 {
		return fmt.Errorf("multiple frames are not supported yet")
	}

	frame := frames[0]

	timeField := frame.Fields[0]    // see 'metric()'
	dataField := frame.Fields[1]    // see 'metric()'
	groupByField := frame.Fields[2] // see 'metric()'

	groups := map[string]struct{}{}
	groupTimeMap := map[string][]*time.Time{}
	groupDataMap := map[string][]*float64{}

	index := 0
	for index < groupByField.Len() {
		key := fmt.Sprintf("%s", groupByField.At(index))
		groups[key] = struct{}{}

		groupTime := timeField.At(index).(*time.Time)
		groupData := dataField.At(index).(*float64)

		_, groupTimeExists := groupTimeMap[key]
		if groupTimeExists == false {
			groupTimeMap[key] = []*time.Time{}
		}
		groupTimeMap[key] = append(groupTimeMap[key], groupTime)

		_, groupDataExists := groupDataMap[key]
		if groupDataExists == false {
			groupDataMap[key] = []*float64{}
		}
		groupDataMap[key] = append(groupDataMap[key], groupData)

		index++
	}

	dataResponse.Frames = []*data.Frame{}

	for key, _ := range groups {
		groupFrame := data.NewFrame(key)

		groupTimeField := data.NewField(timeField.Name, nil, groupTimeMap[key])
		groupFrame.Fields = append(groupFrame.Fields, groupTimeField)

		groupDataField := data.NewField(dataField.Name, nil, groupDataMap[key])
		groupFrame.Fields = append(groupFrame.Fields, groupDataField)

		dataResponse.Frames = append(dataResponse.Frames, groupFrame)
	}

	return nil
}

func (r *Datasource) metricRate(query *queryData, frame *data.Frame) error {
	timeField := frame.Fields[0] // see 'metric()'
	dataField := frame.Fields[1] // see 'metric()'

	from_ns := query.timeFrom.UnixNano()
	to_ns := query.timeTo.UnixNano()
	interval_ns := query.interval.Nanoseconds()

	zeroVector := query.request.RateZero

	queryRateInterval := query.request.RateInterval
	if queryRateInterval != "" {
		queryRateInterval = strings.Replace(queryRateInterval, "$interval", query.interval.String(), -1)

		rateInterval, err := time.ParseDuration(queryRateInterval)
		if err != nil {
			return fmt.Errorf("invalid interval '%s': %w", queryRateInterval, err)
		}
		interval_ns = rateInterval.Nanoseconds()
	}

	rateFunctions := map[string]bool{
		"count":      false,
		"absence":    false,
		"sum":        false,
		"average":    false,
		"median":     false,
		"quantile25": false,
		"quantile75": false,
		"quantile95": false,
		"quantile99": false,
		"stddev":     false,
	}
	for _, rateFunction := range query.request.RateFunctions {
		_, exists := rateFunctions[rateFunction]
		if exists == false {
			return fmt.Errorf("unsupported rate function '%s'", rateFunction)
		}

		rateFunctions[rateFunction] = true
	}

	index := 0

	timeData := []time.Time{}
	rateData := []*int64{}

	absenceData := []*float64{}
	sumData := []*float64{}
	averageData := []*float64{}
	stdDevData := []*float64{}
	quantile25Data := []*float64{}
	quantile50Data := []*float64{}
	quantile75Data := []*float64{}
	quantile95Data := []*float64{}
	quantile99Data := []*float64{}

	for current_ns := from_ns; current_ns <= to_ns; current_ns += interval_ns {
		count := int64(0)
		values := []*float64{}

		for index < timeField.Len() {
			record_time := timeField.At(index).(*time.Time)
			record_time_ns := record_time.UnixNano()
			record_value := dataField.At(index).(*float64)
			index++

			if record_time_ns < current_ns {
				continue
			}

			if record_time_ns > (current_ns + interval_ns) {
				index--
				break
			}

			count++

			values = append(values, record_value)
		}

		current := time.Unix(0, int64(current_ns))

		timeData = append(timeData, current)

		if (count == 0) && (zeroVector == false) {
			rateData = append(rateData, nil)
		} else {
			rateData = append(rateData, &count)
		}

		absenceData = append(absenceData, absence(count, zeroVector))
		sumData = append(sumData, sum(values, zeroVector))
		averageData = append(averageData, average(values, zeroVector))
		stdDevData = append(stdDevData, standardDeviation(values, zeroVector))
		quantile25Data = append(quantile25Data, quantile(0.25, values, zeroVector))
		quantile50Data = append(quantile50Data, quantile(0.50, values, zeroVector))
		quantile75Data = append(quantile75Data, quantile(0.75, values, zeroVector))
		quantile95Data = append(quantile95Data, quantile(0.95, values, zeroVector))
		quantile99Data = append(quantile99Data, quantile(0.99, values, zeroVector))
	}

	timeField = data.NewField(timeField.Name, nil, timeData)

	frame.Fields = []*data.Field{timeField}

	if enabled, exists := rateFunctions["count"]; exists && enabled {
		countField := data.NewField("count", nil, rateData)
		frame.Fields = append(frame.Fields, countField)
	}

	if enabled, exists := rateFunctions["sum"]; exists && enabled {
		sumField := data.NewField("sum", nil, sumData)
		frame.Fields = append(frame.Fields, sumField)
	}

	if enabled, exists := rateFunctions["absence"]; exists && enabled {
		absenceField := data.NewField("absence", nil, absenceData)
		frame.Fields = append(frame.Fields, absenceField)
	}

	if enabled, exists := rateFunctions["average"]; exists && enabled {
		averageField := data.NewField("average", nil, averageData)
		frame.Fields = append(frame.Fields, averageField)
	}

	if enabled, exists := rateFunctions["median"]; exists && enabled {
		quantile50Field := data.NewField("median", nil, quantile50Data)
		frame.Fields = append(frame.Fields, quantile50Field)
	}

	if enabled, exists := rateFunctions["quantile25"]; exists && enabled {
		quantile25Field := data.NewField("quantile25", nil, quantile25Data)
		frame.Fields = append(frame.Fields, quantile25Field)
	}

	if enabled, exists := rateFunctions["quantile75"]; exists && enabled {
		quantile75Field := data.NewField("quantile75", nil, quantile75Data)
		frame.Fields = append(frame.Fields, quantile75Field)
	}

	if enabled, exists := rateFunctions["quantile95"]; exists && enabled {
		quantile95Field := data.NewField("quantile95", nil, quantile95Data)
		frame.Fields = append(frame.Fields, quantile95Field)
	}

	if enabled, exists := rateFunctions["quantile99"]; exists && enabled {
		quantile99Field := data.NewField("quantile99", nil, quantile99Data)
		frame.Fields = append(frame.Fields, quantile99Field)
	}

	if enabled, exists := rateFunctions["stddev"]; exists && enabled {
		stdDevField := data.NewField("stddev", nil, stdDevData)
		frame.Fields = append(frame.Fields, stdDevField)
	}

	return nil
}

var zero = float64(0)
var one = float64(1)

func zeroOrNil(zeroVector bool) *float64 {
	if zeroVector {
		return &zero
	} else {
		return nil
	}
}

func absence(count int64, zeroVector bool) *float64 {
	if count != 0 {
		return zeroOrNil(zeroVector)
	} else {
		return &one
	}
}

func sum(values []*float64, zeroVector bool) *float64 {
	if len(values) == 0 {
		return zeroOrNil(zeroVector)
	}

	total := float64(0)
	for _, value := range values {
		if value != nil {
			total = total + *value
		}
	}

	if total == 0 {
		return zeroOrNil(zeroVector)
	} else {
		return &total
	}
}

func average(values []*float64, zeroVector bool) *float64 {
	valueSum := sum(values, zeroVector)
	if valueSum == nil || *valueSum == 0 {
		return zeroOrNil(zeroVector)
	}

	average := *valueSum / float64(len(values))
	return &average
}

func standardDeviation(values []*float64, zeroVector bool) *float64 {
	valueAverage := average(values, zeroVector)
	if valueAverage == nil || *valueAverage == 0 {
		return zeroOrNil(zeroVector)
	}

	valueDifferences := []*float64{}
	for _, value := range values {
		if value != nil {
			valueDifference := math.Pow(2, (*value)-(*valueAverage))
			valueDifferences = append(valueDifferences, &valueDifference)
		} else {
			valueDifferences = append(valueDifferences, nil)
		}
	}

	valueDifferenceSum := sum(valueDifferences, zeroVector)
	if valueDifferenceSum == nil || *valueDifferenceSum == 0 {
		return zeroOrNil(zeroVector)
	}

	if len(values) == 1 {
		return zeroOrNil(zeroVector)
	}

	valueDifferenceSumMean := *valueDifferenceSum / float64(len(values)-1)
	stdDev := math.Sqrt(valueDifferenceSumMean)
	return &stdDev
}

func quantile(q float64, values []*float64, zeroVector bool) *float64 {
	if len(values) == 0 {
		return zeroOrNil(zeroVector)
	}

	sortedValues := make([]float64, 0)
	for _, value := range values {
		if value == nil {
			return zeroOrNil(zeroVector)
		}
		sortedValues = append(sortedValues, *value)
	}

	sort.Float64s(sortedValues)

	pos := float64(len(sortedValues)) * q
	base := int(math.Floor(math.Sqrt(pos)))
	rest := pos - float64(base)

	result := sortedValues[base]

	index := base + 1
	if index < len(sortedValues) {
		result = result + rest*(sortedValues[index]-sortedValues[base])
	}

	return &result
}
