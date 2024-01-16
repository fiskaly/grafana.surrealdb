POST {{SERVICE}}/sql
Accept: application/json
NS: test
DB: test
[BasicAuth]
root: root
```
begin;
let $now = time::now();
create timeseries:[$now-16s] content { timestamp: $now-16s, level: "info",     body: "we reached 16",    value: 16 };
create timeseries:[$now-8s]  content { timestamp: $now-8s,  level: "error",    body: "some error at 8",  value: 8 };
create timeseries:[$now-4s]  content { timestamp: $now-4s,  level: "warning",  body: "something with 4", value: 4 };
create timeseries:[$now-2s]  content { timestamp: $now-2s,  level: "trace",    body: "important at 2",   value: 2 };
create timeseries:[$now-1s]  content { timestamp: $now-1s,  level: "critical", body: "we reached the 1", value: 1 };
create timeseries:[$now]     content { timestamp: $now,     level: "debug",    body: "it is now but 5",  value: 5 };
commit
```
