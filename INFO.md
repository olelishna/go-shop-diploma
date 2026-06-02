# Run test accrual system

```bash
./cmd/accrual/accrual_linux_amd64 -a=localhost:8091 -d=postgres://gophermart:q1w2e3r4t5@localhost/gophermart?sslmode=disable
```

# Run external tests

```bash
gophermarttest -test.v -test.run=^TestGophermart$ \
-gophermart-binary-path=cmd/gophermart/gophermart \
-gophermart-host=localhost \
-gophermart-port=8080 \
-gophermart-database-uri="postgres://gophermart:q1w2e3r4t5@localhost/gophermart?sslmode=disable" \
-accrual-binary-path=./cmd/accrual/accrual_linux_amd64 \
-accrual-host=localhost \
-accrual-port=8091 \
-accrual-database-uri="postgres://gophermart:q1w2e3r4t5@localhost/gophermart?sslmode=disable"
```