POST {{SERVICE}}/sql
Accept: application/json
NS: test
DB: test
[BasicAuth]
root: root
```
begin;
define user username on database password 'password' roles viewer;
let $now = time::now();
create timeseries:[$now-22s] content { timestamp: $now-22s, level: "info",     body: "this can be 5",    value: 5 };
create timeseries:[$now-21s] content { timestamp: $now-21s, level: "info",     body: "sometimes a 7",    value: 7 };
create timeseries:[$now-20s] content { timestamp: $now-20s, level: "info",     body: "and it is a 6",    value: 6 };
create timeseries:[$now-17s] content { timestamp: $now-17s, level: "info",     body: "there was a 7",    value: 7 };
create timeseries:[$now-16s] content { timestamp: $now-16s, level: "info",     body: "we reached 16",    value: 16 };
create timeseries:[$now-8s]  content { timestamp: $now-8s,  level: "error",    body: "some error at 8",  value: 8 };
create timeseries:[$now-4s]  content { timestamp: $now-4s,  level: "warning",  body: "something with 4", value: 4 };
create timeseries:[$now-2s]  content { timestamp: $now-2s,  level: "trace",    body: "important at 2",   value: 2 };
create timeseries:[$now-1s]  content { timestamp: $now-1s,  level: "critical", body: "we reached the 1", value: 1 };
create timeseries:[$now]     content { timestamp: $now,     level: "debug",    body: "it is now but 5",  value: 5 };
commit
```
