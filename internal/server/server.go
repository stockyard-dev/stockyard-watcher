package server

import (
	"encoding/json"
	"github.com/stockyard-dev/stockyard-watcher/internal/store"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type Server struct {
	db      *store.DB
	mux     *http.ServeMux
	limits  Limits
	dataDir string
	pCfg    map[string]json.RawMessage
}

func New(db *store.DB, limits Limits, dataDir string) *Server {
	s := &Server{db: db, mux: http.NewServeMux(), limits: limits, dataDir: dataDir}
	s.mux.HandleFunc("GET /api/watches", s.listWatches)
	s.mux.HandleFunc("POST /api/watches", s.createWatch)
	s.mux.HandleFunc("GET /api/watches/{id}", s.getWatch)
	s.mux.HandleFunc("DELETE /api/watches/{id}", s.deleteWatch)
	s.mux.HandleFunc("POST /api/watches/{id}/toggle", s.toggleWatch)
	s.mux.HandleFunc("POST /api/changes", s.recordChange)
	s.mux.HandleFunc("GET /api/watches/{id}/changes", s.listChanges)
	s.mux.HandleFunc("GET /api/stats", s.stats)
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /ui", s.dashboard)
	s.mux.HandleFunc("GET /ui/", s.dashboard)
	s.mux.HandleFunc("GET /", s.root)
	s.mux.HandleFunc("GET /api/tier", func(w http.ResponseWriter, r *http.Request) {
		wj(w, 200, map[string]any{"tier": s.limits.Tier, "upgrade_url": "https://stockyard.dev/watcher/"})
	})
	s.loadPersonalConfig()
	s.mux.HandleFunc("GET /api/config", s.configHandler)
	s.mux.HandleFunc("GET /api/extras/{resource}", s.listExtras)
	s.mux.HandleFunc("GET /api/extras/{resource}/{id}", s.getExtras)
	s.mux.HandleFunc("PUT /api/extras/{resource}/{id}", s.putExtras)
	return s
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }
func wj(w http.ResponseWriter, c int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(c)
	json.NewEncoder(w).Encode(v)
}
func we(w http.ResponseWriter, c int, m string) { wj(w, c, map[string]string{"error": m}) }
func (s *Server) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/ui", 302)
}
func (s *Server) listWatches(w http.ResponseWriter, r *http.Request) {
	wj(w, 200, map[string]any{"watches": oe(s.db.ListWatches())})
}
func (s *Server) createWatch(w http.ResponseWriter, r *http.Request) {
	var wt store.Watch
	json.NewDecoder(r.Body).Decode(&wt)
	if wt.Path == "" {
		we(w, 400, "path required")
		return
	}
	wt.Enabled = true
	s.db.CreateWatch(&wt)
	wj(w, 201, s.db.GetWatch(wt.ID))
}
func (s *Server) getWatch(w http.ResponseWriter, r *http.Request) {
	wt := s.db.GetWatch(r.PathValue("id"))
	if wt == nil {
		we(w, 404, "not found")
		return
	}
	wj(w, 200, wt)
}
func (s *Server) deleteWatch(w http.ResponseWriter, r *http.Request) {
	s.db.DeleteWatch(r.PathValue("id"))
	wj(w, 200, map[string]string{"deleted": "ok"})
}
func (s *Server) toggleWatch(w http.ResponseWriter, r *http.Request) {
	s.db.ToggleWatch(r.PathValue("id"))
	wj(w, 200, s.db.GetWatch(r.PathValue("id")))
}
func (s *Server) recordChange(w http.ResponseWriter, r *http.Request) {
	var c store.Change
	json.NewDecoder(r.Body).Decode(&c)
	if c.WatchID == "" {
		we(w, 400, "watch_id required")
		return
	}
	s.db.RecordChange(&c)
	wj(w, 201, c)
}
func (s *Server) listChanges(w http.ResponseWriter, r *http.Request) {
	wj(w, 200, map[string]any{"changes": oe(s.db.ListChanges(r.PathValue("id"), 50))})
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) { wj(w, 200, s.db.Stats()) }
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	st := s.db.Stats()
	wj(w, 200, map[string]any{"status": "ok", "service": "watcher", "watches": st.Watches, "changes": st.Changes})
}
func oe[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
func init() { log.SetFlags(log.LstdFlags | log.Lshortfile) }

// ─── personalization (auto-added) ──────────────────────────────────

func (s *Server) loadPersonalConfig() {
	path := filepath.Join(s.dataDir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("%s: warning: could not parse config.json: %v", "watcher", err)
		return
	}
	s.pCfg = cfg
	log.Printf("%s: loaded personalization from %s", "watcher", path)
}

func (s *Server) configHandler(w http.ResponseWriter, r *http.Request) {
	if s.pCfg == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.pCfg)
}

func (s *Server) listExtras(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")
	all := s.db.AllExtras(resource)
	out := make(map[string]json.RawMessage, len(all))
	for id, data := range all {
		out[id] = json.RawMessage(data)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) getExtras(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")
	id := r.PathValue("id")
	data := s.db.GetExtras(resource, id)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(data))
}

func (s *Server) putExtras(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")
	id := r.PathValue("id")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"read body"}`, 400)
		return
	}
	var probe map[string]any
	if err := json.Unmarshal(body, &probe); err != nil {
		http.Error(w, `{"error":"invalid json"}`, 400)
		return
	}
	if err := s.db.SetExtras(resource, id, string(body)); err != nil {
		http.Error(w, `{"error":"save failed"}`, 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":"saved"}`))
}
