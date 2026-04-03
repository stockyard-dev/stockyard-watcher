package main
import ("fmt";"log";"net/http";"os";"github.com/stockyard-dev/stockyard-watcher/internal/server";"github.com/stockyard-dev/stockyard-watcher/internal/store")
func main(){port:=os.Getenv("PORT");if port==""{port="8750"};dataDir:=os.Getenv("DATA_DIR");if dataDir==""{dataDir="./watcher-data"}
db,err:=store.Open(dataDir);if err!=nil{log.Fatalf("watcher: %v",err)};defer db.Close();srv:=server.New(db,server.DefaultLimits())
fmt.Printf("\n  Watcher — Self-hosted file change monitor\n  ─────────────────────────────────\n  Dashboard:  http://localhost:%s/ui\n  API:        http://localhost:%s/api\n  Data:       %s\n  ─────────────────────────────────\n\n",port,port,dataDir)
log.Printf("watcher: listening on :%s",port);log.Fatal(http.ListenAndServe(":"+port,srv))}
