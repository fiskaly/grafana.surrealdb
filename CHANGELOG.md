# Changelog

## 0.3.0

Provided dashboard variable-based query support.

## 0.2.0

Provided new `groupBy` functionality in `metric` query mode in order to perform data grouping for plain `metric` values as well as `rate` based functions.
Updated plugin documentation and screenshots.

## 0.1.2

Enabled Grafana policy token for signing the data source plugin in order to be published in the Grafana plugin catalog.

## 0.1.1

Added additional optional `scope` for data source configuration.
Provided the data source plugin handling to respect the `hide` query option.
Improved internal stability of the plugin backend.

## 0.1.0

Initial release.
Provides basic data source connection to a SurrealDB instance through the configuration of `location`, `namespace`, `database`, `username`, and `password`.
The query editor provides a `raw`, `log`, and `metric` mode.
