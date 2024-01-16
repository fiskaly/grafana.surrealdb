define function fn::rate
( $interval: duration
, $from: string
, $to: string
, $options: object
, $query: array
) {
    // maybe use FOR statement when array::map feature is available 
    // see: https://github.com/surrealdb/surrealdb/issues/2814
    return function(duration::millis($interval), $from, $to, $options, $query) {
        let results = [];
        let interval_ms = arguments[0]
        var from_ms = new Date(arguments[1]).valueOf()
        var to_ms = new Date(arguments[2]).valueOf()
        if( from_ms > to_ms ) {
            let tmp = from_ms
            from_ms = to_ms
            to_ms = tmp
        }

        let options = arguments[3] 
        var config = {}
        config.time           = (options?.time     !== undefined) ? options.time     : {}
        config.time.key       = (options.time?.key !== undefined) ? options.time.key : "timestamp"

        config.value          = (options?.value          !== undefined) ? options.value         : {}
        config.value.key      = (options.value?.key      !== undefined) ? options.value.key     : "value"
        config.value.sum      = (options.value?.sum      !== undefined) ? options.value.sum      : false
        config.value.average  = (options.value?.average  !== undefined) ? options.value.average  : false
        config.value.stddev   = (options.value?.stddev   !== undefined) ? options.value.stddev   : false
        config.value.median   = (options.value?.median   !== undefined) ? options.value.median   : false
        config.value.quantile = (options.value?.quantile !== undefined) ? options.value.quantile : []
        config.value.absence  = (options.value?.absence  !== undefined) ? options.value.absence  : false
        config.value.zero     = (options.value?.zero     !== undefined) ? options.value.zero     : false

        let query = arguments[4]
        if( query.length == 0 ) {
            query = []
        } else {
            let query_from = query[0][config.time.key]
            let query_to = query[query.length-1][config.time.key]
            if( query_from > query_to ) {
                query.reverse()
            }
        }

        let zeroOrNull = (isZero) => isZero ? 0 : null

        let absence = (count, isZero) => ( count > 0 ) ? zeroOrNull(isZero) : 1

        let sort = (values) => values.sort((x, y) => x - y)

        let sum = (values) => values.reduce((x, y) => x + y, 0)

        let average = (values) => sum(values) / values.length

        let standardDeviation = (values) => {
            let valueAverage = average(values);
            let valueDifference = values.map(x => (x - valueAverage) ** 2);
            return Math.sqrt(sum(valueDifference) / (values.length - 1));
        }

        let quantile = (values, q) => {
            let sorted = sort(values);
            let pos = (sorted.length - 1) * q;
            let base = Math.floor(pos);
            let rest = pos - base;
            if (sorted[base + 1] !== undefined) {
                return sorted[base] + rest * (sorted[base + 1] - sorted[base]);
            } else {
                return sorted[base];
            }
        }

        var index = 0

        for( let current_ms = from_ms; current_ms <= to_ms; current_ms += interval_ms )
        {
            var count = 0
            var values = []

            while( index < query.length ) {
                let record = query[index]
                let record_ms = record[config.time.key].valueOf()
                index++

                if( record_ms < current_ms ) {
                    continue
                }

                if( record_ms > ( current_ms + interval_ms ) ) {
                    index--
                    break
                }

                count++
                values.push( record[config.value.key] )
            }

            var result =
            { timestamp: new Date(current_ms).toISOString()
              , count: ( count > 0 ) ? count : zeroOrNull(config.value.zero)
            }
            if( config.value.absence ) {
                result.absence = absence(count, config.value.zero)
            }
            if( config.value.sum ) {
                result.sum = sum(values) || zeroOrNull(config.value.zero)
            }
            if( config.value.average ) {
                result.average = average(values) || zeroOrNull(config.value.zero)
            }
            if( config.value.stddev ) {
                result.stddev = standardDeviation(values) || zeroOrNull(config.value.zero)
            }
            if( config.value.median ) {
                result.median = quantile(values, .50) || zeroOrNull(config.value.zero)
            }
            for( let i = 0; i < config.value.quantile.length; i++ ) {
                let q_i = config.value.quantile[i]
                result[ 'quantile' + q_i.toString() ] = quantile(values, q_i / 100.0) || zeroOrNull(config.value.zero)
            }

            results.push( result )
        }

        return results
    }
}

// return fn::grafana::rate
// ( 10s
// , '2024-01-09T15:16:00Z'
// , '2024-01-09T15:16:40Z'
// , { value : { absence : true 
//             , sum : true 
//             , average : true 
//             , stddev : true 
//             , median : true 
//             , quantile : [ 95, 99 ] 
//             }
//   }
// , select * // timestamp, value
//   from foo:['2024-01-09T15:16:08Z']..['2024-01-09T15:16:39Z']
// )

