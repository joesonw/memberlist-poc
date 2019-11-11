Sample guide for hashicopr/memberlist

Run:

./main -port=9000 -name=node1 -listen=:9080
./main -port=9100 -name=node2 -listen=:9180 -peers=localhost:9000
./main -port=9200 -name=node3 -listen=:9280 -peers=localhost:9100

Available commands:

list `curl http://localhost:PORT`

insert `curl -X POST http://localhost:PORT -d '{"id":1,"name":"abc"}'
