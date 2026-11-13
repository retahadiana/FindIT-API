package main

import (
"bytes"
"flag"
"fmt"
"net/http"
"sync"
"sync/atomic"
)

func main() {
url := flag.String("url", "http://localhost:3000/api/v1/claims/1/status", "endpoint approve klaim")
token := flag.String("token", "", "Bearer token penemu barang")
workers := flag.Int("n", 20, "jumlah goroutine serentak")
flag.Parse()

var wg sync.WaitGroup
var ok200, conflict409, other atomic.Int32
client := &http.Client{}
payload := []byte(`{"status":"approved"}`)

for i := 0; i < *workers; i++ {
wg.Add(1)
go func() {
defer wg.Done()
req, _ := http.NewRequest(http.MethodPatch, *url, bytes.NewReader(payload))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Authorization", "Bearer "+*token)
resp, err := client.Do(req)
if err != nil {
other.Add(1)
return
}
resp.Body.Close()
switch resp.StatusCode {
case http.StatusOK:
ok200.Add(1)
case http.StatusConflict:
conflict409.Add(1)
default:
other.Add(1)
}
}()
}
wg.Wait()
fmt.Printf("Hasil OCC Load Test -> 200 OK: %d | 409 Conflict: %d | Lainnya: %d\n",
ok200.Load(), conflict409.Load(), other.Load())
}

