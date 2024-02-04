
# Surrealdb

This Grafana data source plugin provides the ability to connect, query, and visualize [SurrealDB](https://surrealdb.com/) stored information.

## Requirements

Besides the already installed and setup Grafana service -- for minimal version see plugin dependencies -- the following requirements must be met in order to use this plugin:

* a deployed SurrealDB [instance](https://docs.surrealdb.com/docs/deployment/overview) which is accessible through the network of your the Grafana service where this plugin will be installed

* an existing SurrealDB [namespace](https://docs.surrealdb.com/docs/surrealql/statements/define/namespace)

* an existing SurrealDB [database](https://docs.surrealdb.com/docs/surrealql/statements/define/database)

* an optional existing SurrealDB [scope](https://docs.surrealdb.com/docs/surrealql/statements/define/scope)

* an existing SurrealDB [user](https://docs.surrealdb.com/docs/security/overview) which has access to the namespace, database, and optionally the scope -- recommendation is to use a SurrealDB user which has the `VIEWER`
[role](https://docs.surrealdb.com/docs/surrealql/statements/define/user#roles) in order to perform exclusively read-only operations and do not alter any database state to maintain the data integrity

Since the plugin is used to query the contents of one or multiple SurrealDB [tables](https://docs.surrealdb.com/docs/surrealql/statements/define/table), the existence of the tables in question is not required per default, because the plugin can be installed without an existing table and e.g. query and visualize database [information](https://docs.surrealdb.com/docs/surrealql/statements/info) as well.

## Installation

The plugin can be installed through the user interface in the Grafana service itself or by using the command-line utility via:

```
$ grafana-cli plugins install fiskaly-surrealdb-datasource
```

If the Grafana service was setup in a containerized context via e.g. `docker-compose` or started locally on a machine the plugin can be installed automatically for the Grafana instance during start-up by setting the following environment variable:

```
GF_INSTALL_PLUGINS="fiskaly-surrealdb-datasource"
```

## Configuration

Following the [data source management](https://grafana.com/docs/grafana/latest/administration/data-source-management/) documentation and selecting this plugin in the [add new data source connection](https://grafana.com/docs/grafana/latest/datasources/add-a-data-source/) page, the configuration of this plugin requires the following parameters to be setup:

* `Location` of the [WebSocket](https://docs.surrealdb.com/docs/integration/websocket#signin) connection in form of a `address:port` schema (required, default value: `localhost:8000`)

* `Namespace` to [use](https://docs.surrealdb.com/docs/surrealql/statements/define/namespace) (required, default value: `default`)

* `Database` to [use](https://docs.surrealdb.com/docs/surrealql/statements/define/database) (required, default value: `default`)

* `Scope` to [use](https://docs.surrealdb.com/docs/surrealql/statements/define/scope) (optional)

* `Username` for the [user](https://docs.surrealdb.com/docs/surrealql/statements/define/user) to interact with the SurrealDB instance
(required, default value: `root`)

* `Password` of the provided [user](https://docs.surrealdb.com/docs/surrealql/statements/define/user) to perform the  [authentication](https://docs.surrealdb.com/docs/security/authentication)
(required, default value: `root`)

---

![config](https://github.com/fiskaly/grafana.surrealdb/assets/6830431/ea076c74-a959-4363-8aed-a5797358a28e)

## Usage

The plugin defines three query modes: `Raw`, `Log`, and `Metric`
For all modes the actual query is written in [SurrealQL](https://docs.surrealdb.com/docs/surrealql/overview/).
Therefore, the `Raw` mode is representing the query results in a table view as preferred visualization type.

The `Log` mode changes the preferred visualization type to log-based view and allows to define/set the log `Time` and optional log `Message` column information.

For time series value-based visualizations, the plugin provides a `Metric` mode to represent the query results in a graph view as preferred visualization type.
This mode allows to further configure/set the actual `Data` column to visualize the time series.
Furthermore, based on the `Data` column there is an option to perform data grouping of a given `Field` as well as to perform different `Rate` computations in a given `Interval`.

---

![query](https://github.com/fiskaly/grafana.surrealdb/assets/6830431/77f47494-1815-48bc-8e40-ff43822bc68d)


## Design

The plugin consists of a frontend and a backend part.
The frontend is written in TypeScript and provides two major components:
(1) a `ConfigEditor` to setup and connect to a SurrealDB instance; and
(2) a `QueryEditor` to write queries in SurrealDB Query Language ([SurrealQL](https://docs.surrealdb.com/docs/surrealql/overview/)), configure the query mode, and interact with the backend part of the data source plugin.

The backend is written in Golang and provides a low-level connection through the SurrealDB [WebSocket](https://docs.surrealdb.com/docs/integration/websocket) interface.
After the successful connection and [signin](https://docs.surrealdb.com/docs/integration/websocket/#signin) operation, all queries send from the frontend part to the backend part are directly executed through the WebSocket connection using the [custom query](https://docs.surrealdb.com/docs/integration/websocket#query) operation.

Since SurrealDB as well as Grafana support [variables](https://grafana.com/docs/grafana/latest/dashboards/variables/) the plugin supports and performs the following variable resolving steps:
(1) in the frontend part are all Grafana scope provided variables replaced; and
(2) in the backend part defines the following plugin specific variables and replaces those before the query is send and executed on the SurrealDB instance:

- `$interval` defined by the current query editor context
- `$now` current timestamp in UTC taken at the beginning of the query execution
- `$from` starting time as UTC timestamp of the current query editor context
- `$to`  ending time as UTC timestamp of the current query editor context
