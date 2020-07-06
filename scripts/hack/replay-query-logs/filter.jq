. |
select( .ts > "2020-07-01T16:30") | # TODO take timestamp as param?
select( .params.step != 0) | # step=0 is query instead of query_range (I think?)
@uri "https://$METRIC_STORE_DOMAIN/api/v1/query_range?query=\(.params.query)&start=\(.params.start)&end=\(.params.end)&step=\(.params.step)"



