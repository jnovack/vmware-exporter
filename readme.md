# Prometheus vmware exporter in golang

**Up and running in 3 Steps**

1 - Build:
```
$ git clone https://github.com/marstid/go-vmware-exporter.git
$ cd go-vmware-exporter/
$ docker build -t go-vm -f Dockerfile .
```

2 - Edit docker-compose.yml to configure vCenter host and credentials.
```
$ vi docker-compose.yml
```

3 - Start 
```
$ docker-compose up -d
```


Curl http://localhost:9094