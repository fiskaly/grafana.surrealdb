import React,
{ ChangeEvent
} from 'react';

import
{ InlineField
, Select
, InlineSwitch
, QueryField
, VerticalGroup
, HorizontalGroup
} from '@grafana/ui';

import
{ QueryEditorProps
, SelectableValue
} from '@grafana/data';

import
{ DataSource
} from '../datasource';

import
{ MyDataSourceOptions
  , MyQuery
} from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
    const
    { mode
    , surql
    , requery
    , timestamp
    , logMessage
    , metricData
    , rate
    , rateZero
    , rateInterval
    , rateFunctions
    } = query;

    return (
<div className="gf-form">
  <VerticalGroup>
    <HorizontalGroup>
      <InlineField
        label="Type"
        labelWidth={12}
        tooltip="Query type and preferred visualization."
      >
      <Select
        className={"className"}
        isMulti={false}
        isClearable={false}
        backspaceRemovesValue={false}
        onChange={(selected: SelectableValue<string>) => {
            onChange({ ...query, mode: selected.value });
            onRunQuery();
        }}
        options={
            [ { value: "raw", label: "Raw" }
            , { value: "log", label: "Logs" }
            , { value: "metric", label: "Metric" }
            ]
        }
        isSearchable={false}
        maxMenuHeight={500}
        placeholder={"placeholder"}
        noOptionsMessage={"No options found"}
        value={ mode || "raw" }
        width={14}
      />
      </InlineField>
      <InlineField
        label="Requery"
        labelWidth={12}
        tooltip="Requery automatically after changing any given parameter in the query editor."
      >
      <div style={{ minWidth: 190 }}>
      <InlineSwitch
        value={requery}
        disabled={false}
        transparent={false}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
            let checked = event.target.checked;
            if( checked ) {
                onRunQuery();    
            }
            onChange({ ...query, requery: checked });
        }}
      />
      </div>
      </InlineField>
      </HorizontalGroup>
      <VerticalGroup>
      <InlineField
        label="Query"
        labelWidth={12}
        tooltip="SurrealDB Query Language (QL)"
      >
      <div style={{ minWidth: 600 }}>
      <QueryField
        placeholder={"select * from table"}
        portalOrigin="SurrealDB"
        query={surql}
        disabled={false}
        onChange={(value: string) => {
            onChange({ ...query, surql: value });
            if( requery ) { 
                onRunQuery();
            }
        }}
        onBlur={() => {}}
      />
      </div>
      </InlineField>
      </VerticalGroup>
      <HorizontalGroup>
{ (mode === "log" || mode === "metric") &&
      <InlineField
        label="Time"
        labelWidth={12}
        tooltip="Timeseries timestamp field."
      >
      <div style={{ minWidth: 245 }}>
      <QueryField
        placeholder={"timestamp"}
        portalOrigin=""
        query={timestamp}
        disabled={false}
        onChange={(value: string) => {
            onChange({ ...query, timestamp: value });
            if( requery ) {
                onRunQuery();
            }
        }}
        onBlur={() => {}}
      />
      </div>
      </InlineField>
}
{ (mode === "log" ) &&
      <InlineField
        label="Message"
        labelWidth={12}
        tooltip="Log message body field."
      >
      <div style={{ minWidth: 245 }}>
      <QueryField
        placeholder={"body"}
        portalOrigin=""
        query={logMessage}
        disabled={false}
        onChange={(value: string) => {
            onChange({ ...query, logMessage: value });
            if( requery ) {
                onRunQuery();
            }
        }}
        onBlur={() => {}}
      />
      </div>
      </InlineField>
}
{ (mode === "metric") &&
      <InlineField
        label="Data"
        labelWidth={12}
        tooltip="Metric data value field."
      >
      <div style={{ minWidth: 245 }}>
      <QueryField
        placeholder={"value"}
        portalOrigin=""
        query={metricData}
        disabled={false}
        onChange={(value: string) => {
            onChange({ ...query, metricData: value });
            if( requery ) {
                onRunQuery();
            }
        }}
        onBlur={() => {}}
          />
          </div>
        </InlineField>
}
      </HorizontalGroup>
      <HorizontalGroup>
{ (mode === "metric") &&
      <InlineField
        label="Rate"
        labelWidth={12}
        tooltip="Enable rate computation."
      >
      <InlineSwitch
        value={rate}
        disabled={false}
        transparent={false}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
            let checked = event.target.checked;
            onChange({ ...query, rate: checked });
            if( requery ) {
                onRunQuery();
            }
        }}
      />
      </InlineField>
}
{ (mode === "metric") && rate &&
      <InlineField
        label="Zero Vector"
        labelWidth={14}
        tooltip="Convert the null vector to a zero vector."
      >
      <div style={{ minWidth: 68 }}>
      <InlineSwitch
        value={rateZero}
        disabled={false}
        transparent={false}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
            let checked = event.target.checked;
            onChange({ ...query, rateZero: checked });
            if( requery ) {
                onRunQuery();
            }
        }}
      />
      </div>
      </InlineField>
}
{ (mode === "metric") && rate &&
      <InlineField
        label="Interval"
        labelWidth={12}
        tooltip="Rate interval, by default it's using the value of the parameter $interval."
      >
      <div style={{ minWidth: 245 }}>
      <QueryField
        placeholder={""}
        portalOrigin=""
        query={rateInterval}
        disabled={false}
        onChange={(value: string) => {
            onChange({ ...query, rateInterval: value });
            if( requery ) {
                onRunQuery();
            }
        }}
        onBlur={() => {}}
      />
      </div>
      </InlineField>
}
      </HorizontalGroup>
      <HorizontalGroup>
{ (mode === "metric") && rate &&
      <InlineField
        label="Function"
        labelWidth={12}
        tooltip="Apply rate functions to the data value."
      >
      <Select
        isMulti={true}
        isClearable={true}
        backspaceRemovesValue={false}
        onChange={(selected: SelectableValue<string>) => {
            onChange({ ...query, rateFunctions: selected.map( (v: SelectableValue<string>) => v.value ) });
            if( requery ) {
                onRunQuery();
            }
        }}
        options={
            [ { value: "absence", label: "Absence" }
            , { value: "average", label: "Average" }
            , { value: "median", label: "Median" }
            , { value: "sum", label: "Sum" }
            , { value: "stddev", label: "StdDev" }
            , { value: "quantile25", label: "Quantile25" }
            , { value: "quantile75", label: "Quantile75" }
            , { value: "quantile95", label: "Quantile95" }
            , { value: "quantile99", label: "Quantile99" }
            ]
        }
        isSearchable={true}
        maxMenuHeight={500}
        placeholder={""}
        noOptionsMessage={"No options found"}
        value={ rateFunctions }
        width={75.5}
      />
    </InlineField>
}
    </HorizontalGroup>
  </VerticalGroup>
</div>
  );
}
