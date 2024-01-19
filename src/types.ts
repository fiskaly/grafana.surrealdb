import
{ DataSourceJsonData
} from '@grafana/data';

import
{ DataQuery
} from '@grafana/schema';

export interface MyQuery extends DataQuery {
    mode?: string;
    surql: string;
    requery: boolean;
    timestamp: string;
    logMessage: string;
    metricData: string;
    rate: boolean;
    rateZero: boolean;
    rateInterval: string;
    rateFunctions: string[];
}

export const DEFAULT_QUERY: Partial<MyQuery> =
{ mode: "raw"
, surql: "info for namespace"
, requery: true
, timestamp: ""
, logMessage: ""
, metricData: ""
, rate: false
, rateZero: false
, rateInterval: ""
, rateFunctions: []
};

/**
 * These are options configured for each DataSource instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
    location?: string;
    nameaddr?: string;
    database?: string;
    username?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
    password?: string;
}
