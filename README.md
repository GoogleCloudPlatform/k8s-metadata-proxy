# k8s-metadata-proxy

This repo contains a simple proxy for serving concealed metadata to container
workloads running in kubernetes/kubernetes on a GCE VM.

## Performance

This proxy has been benchmarked at requiring no more than 25Mi memory.  With
such a constraint and effectively no cpu constraint, it can serve 200
concurrent requests indefinitely at around 700 qps:

```
$ kubectl describe pod metadata-proxy-v0.1-xxxxx
[...]
Containers:
  metadata-proxy:
    [...]
    Limits:
      cpu: 500m
      memory: 25Mi
    Requests:
      cpu: 500m
      memory: 25Mi
[...]

$ ab -n 200000 -c 200 -H 'Metadata-Flavor:Google' http://127.0.0.1:988/computeMetadata/v1/instance/service-accounts/default/token
This is ApacheBench, Version 2.3 <$Revision: 1604373 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

[...]

Server Software:        Metadata
Server Hostname:        127.0.0.1
Server Port:            988

Document Path:          /computeMetadata/v1/instance/service-accounts/default/token
Document Length:        202 bytes

Concurrency Level:      200
Time taken for tests:   251.792 seconds
Complete requests:      200000
Failed requests:        0
Total transferred:      86000000 bytes
HTML transferred:       40400000 bytes
Requests per second:    794.31 [#/sec] (mean)
Time per request:       251.792 [ms] (mean)
Time per request:       1.259 [ms] (mean, across all concurrent requests)
Transfer rate:          333.55 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    0  17.5      0    1003
Processing:    42  251  60.5    247     959
Waiting:       42  251  60.6    246     958
Total:         42  252  62.5    247    1212

Percentage of the requests served within a certain time (ms)
  50%    247
  66%    271
  75%    288
  80%    298
  90%    329
  95%    357
  98%    396
  99%    423
 100%   1212 (longest request)
```

Under cpu constraint, the qps goes down to about 50, but the pod serves all requests successfully:

```
$ kubectl describe pod metadata-proxy-v0.1-xxxxx
[...]
Containers:
  metadata-proxy:
    [...]
    Limits:
      cpu: 30m
      memory: 25Mi
    Requests:
      cpu: 30m
      memory: 25Mi
[...]

$ ab -n 200000 -c 200 -H 'Metadata-Flavor:Google' http://127.0.0.1:988/computeMetadata/v1/instance/service-accounts/default/token
This is ApacheBench, Version 2.3 <$Revision: 1604373 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

[...]

Server Software:        Metadata
Server Hostname:        127.0.0.1
Server Port:            988

Document Path:          /computeMetadata/v1/instance/service-accounts/default/token
Document Length:        202 bytes

Concurrency Level:      200
Time taken for tests:   3592.015 seconds
Complete requests:      200000
Failed requests:        0
Total transferred:      86000000 bytes
HTML transferred:       40400000 bytes
Requests per second:    55.68 [#/sec] (mean)
Time per request:       3592.015 [ms] (mean)
Time per request:       17.960 [ms] (mean, across all concurrent requests)
Transfer rate:          23.38 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    1  32.4      0    1004
Processing:   892 3590 633.2   3593    8102
Waiting:      892 3581 630.2   3504    7999
Total:        899 3591 634.3   3595    8102

Percentage of the requests served within a certain time (ms)
  50%   3595
  66%   3798
  75%   3901
  80%   4000
  90%   4300
  95%   4602
  98%   5200
  99%   5699
 100%   8102 (longest request)
```

Above 200 concurrent requests, it starts resetting connections, but does not go
above 25MiB memory.
