import
{ DataSourceInstanceSettings
, DataQueryRequest
, DataQueryResponse
, CoreApp
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
	this.getResource('constructor');
    }

    getDefaultQuery(coreApp: CoreApp): Partial<MyQuery> {
	this.getResource('getDefaultQuery');
	return DEFAULT_QUERY;
    }

    query(request: DataQueryRequest<MyQuery>): Observable<DataQueryResponse> {
	this.getResource('query');
	return super.query(request);
    }

    // https://grafana.com/docs/grafana/latest/dashboards/variables/add-template-variables/#global-variables
    applyTemplateVariables(query: MyQuery) {//, scopedVars: ScopedVars)) {
	const templateSrv = getTemplateSrv();
	return {
	    ...query,
	    surql: templateSrv.replace(query.surql),//, scopedVars),
	};
    }
}
