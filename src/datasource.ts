import
{ CoreApp
, DataSourceInstanceSettings
, DataQueryRequest
, DataQueryResponse
, MetricFindValue
, ScopedVars
, TimeRange
} from '@grafana/data';

import
{ DataSourceWithBackend
, getTemplateSrv
} from '@grafana/runtime';

import
{ MyQuery
, MyDataSourceOptions
, DEFAULT_QUERY
} from './types';

import
{ Observable
} from 'rxjs';

export class DataSource extends DataSourceWithBackend<MyQuery, MyDataSourceOptions> {
    constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
	super(instanceSettings);
    }

    getDefaultQuery(coreApp: CoreApp): Partial<MyQuery> {
	return DEFAULT_QUERY;
    }

    query(request: DataQueryRequest<MyQuery>): Observable<DataQueryResponse> {
	return super.query(request);
    }

    filterQuery(query: MyQuery): boolean {
	if (query.hide || query.surql === '') {
	    return false;
	}
	return true;
    }

    // https://grafana.com/docs/grafana/latest/dashboards/variables/add-template-variables/#global-variables
    applyTemplateVariables(query: MyQuery, scopedVars: ScopedVars) {
	return {
	    ...query,
	    surql: getTemplateSrv().replace(query.surql, scopedVars),
	};
    }

    // https://grafana.com/developers/plugin-tools/create-a-plugin/extend-a-plugin/add-support-for-variables#add-support-for-query-variables-to-your-data-source
    metricFindQuery(surql: string, options: { range: TimeRange, scopedVars: ScopedVars, variable: { id: string }}): Promise<MetricFindValue[]> {
	let now = new Date();
	let range = options?.range;
	let scopedVars = options.scopedVars;
	let variableId = options.variable.id;
	let interval = scopedVars?.__interval?.text || "1s"
	let intervalMs = scopedVars?.__intervalMs?.value || 1000

	let query: MyQuery[] =
	[ { refId: variableId
	  , mode: "raw"
	  , surql: surql
	  , requery: false
	  }
	];

	let request: DataQueryRequest<MyQuery> =
	{ requestId: variableId
	, app: "dashboard"
	, timezone: "browser"
	, interval: interval
	, intervalMs: intervalMs
	, range: range
	, startTime: now.getTime()
	, scopedVars: scopedVars
	, targets: query
	};

	let observable = super.query(request)
	return new Promise<MetricFindValue[]>(
	    (myResolve, myReject) => {
		let response: any = {}

		observable.subscribe({
		    next(element) {
			response = element
		    },
		    error(err) {
			myReject([ { "text": "Query failed: " + err } ])
		    },
		    complete() {
			if( response.state === 'Done' ) {
			    let values: any[] = []

			    response.data[0].fields[0].values.forEach(
				(element: any) => {
				    let text = element
				    if( typeof text !== "string" ) {
					text = JSON.stringify(text)
				    }
				    values.push( { "text": text } )
				}
			    );
			    myResolve(values)
			} else if( response.state === 'Error' ) {
			    myReject([{ "text": response.error.message }])
			} else {
			    myReject([{ "text": "Query failed: internal error" }])
			}
		    },
		});
	    }
	);
    }
}
