package main

import (
	"encoding/json"
	"github.com/fsnotify/fsnotify"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type jsonSession struct {
	DebugPath string `json:"debug_path"`
	Epoch     int    `json:"epoch"`
}

type ExpSession struct {
	Name string

	ExpPath   string
	ItemIndex int
	Epoch     int

	f          *os.File
	lastActive time.Time

	exp    *Experiment
	result *Result
}

func NewExpSession(jsonPath string) (*ExpSession, error) {
	f, err := os.Open(jsonPath)
	if err != nil {
		return nil, err
	}

	return &ExpSession{
		Name:       filepath.Base(jsonPath),
		f:          f,
		lastActive: time.Now(),
	}, nil
}

func (s *ExpSession) CheckActive() bool {
	stat, err := s.f.Stat()
	if err != nil {
		return false
	}
	s.lastActive = stat.ModTime()
	return time.Since(s.lastActive).Seconds() < 2
}

func (s *ExpSession) IsExpired() bool {
	if time.Since(s.lastActive).Seconds() > 2 {
		return !s.CheckActive()
	}
	return false
}

func (s *ExpSession) Update() {
	_, err := s.f.Seek(0, io.SeekStart)
	if err != nil {
		log.Println("failed to read session", s.Name, err)
		return
	}

	var session jsonSession
	decoder := json.NewDecoder(s.f)
	err = decoder.Decode(&session)
	if err != nil {
		log.Println("failed to read session", s.Name, err)
		return
	}

	s.Epoch = session.Epoch

	if s.exp == nil {
		s.ExpPath = strings.TrimPrefix(filepath.Dir(session.DebugPath), "exp/")
		s.ItemIndex, _ = matchPcFolder(session.DebugPath)

		exp := baseEntry.FindOrCreate(s.ExpPath)
		if exp == nil {
			log.Println(s.ExpPath, "does not exist")
			return
		}
		s.exp = exp.Experiment
	}

	result := s.exp.GetResult(s.ItemIndex)
	if result == nil {
		log.Println(s.ExpPath, s.ItemIndex, "does not exist")
		return
	}

	result.Refresh()
	result.NotifyPcChanges(s.Epoch)
	result.SetActive(true)
}

func (s *ExpSession) Close() {
	err := s.f.Close()
	if err != nil {
		log.Println("failed to close session", s.Name, err)
	}

	result := s.exp.GetResult(s.ItemIndex)
	if result == nil {
		log.Println(s.ExpPath, s.ItemIndex, "does not exist")
		return
	}
	result.SetActive(false)
}

func (s *ExpSession) Remove() {
	err := os.Remove(s.f.Name())
	if err != nil {
		log.Println("failed to remove session", s.Name, err)
	}
}

type SessionWatcher struct {
	sessions map[string]*ExpSession

	sessionPath string

	Watcher *fsnotify.Watcher

	stopCh chan struct{}

	mapMutex sync.Mutex
}

func NewSessionWatcher(expPath string) *SessionWatcher {
	w := &SessionWatcher{
		sessions:    make(map[string]*ExpSession),
		sessionPath: filepath.Join(expPath, "sessions"),
	}

	go w.Watch()

	return w
}

func (e *SessionWatcher) Watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("Failed to watch: ", err)
		return
	}
	e.Watcher = watcher

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				//log.Printf("Event: %s %d", event.Name, event.Op)
				filename := filepath.Base(event.Name)
				if strings.HasSuffix(filename, ".json") {
					if event.Has(fsnotify.Create) {
						e.handleCreate(filename)
					}
					if event.Has(fsnotify.Write) {
						e.handleWrite(filename)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Error:", err)
			case <-e.stopCh:
				return
			case <-time.After(time.Second * 10):
				e.RemoveExpired()
			}
		}
	}()

	err = watcher.Add(e.sessionPath)
	if err != nil {
		log.Println("Failed to watch: ", err)
		return
	}
	log.Printf("SessionWatcher Watching: %s", e.sessionPath)
}

func (e *SessionWatcher) handleCreate(filename string) {
	e.mapMutex.Lock()
	defer e.mapMutex.Unlock()

	session, ok := e.sessions[filename]
	if ok {
		session.Close()
	}
	var err error
	session, err = NewExpSession(filepath.Join(e.sessionPath, filename))
	if err != nil {
		log.Println("Failed to create session", filename, err)
		return
	}
	e.sessions[filename] = session
	session.Update()
}

func (e *SessionWatcher) handleWrite(filename string) {
	session, ok := e.sessions[filename]
	if !ok {
		var err error
		session, err = NewExpSession(filepath.Join(e.sessionPath, filename))
		if err != nil {
			log.Println("Failed to create session", filename, err)
			return
		}
	}
	session.Update()
}

func (e *SessionWatcher) RemoveExpired() {
	e.mapMutex.Lock()
	defer e.mapMutex.Unlock()

	if len(e.sessions) == 0 {
		return
	}

	var expiredSessions []*ExpSession

	for _, session := range e.sessions {
		if session.IsExpired() {
			expiredSessions = append(expiredSessions, session)
		}
	}
	for _, session := range expiredSessions {
		session.Close()
		session.Remove()
		delete(e.sessions, session.Name)
	}
}

func (e *SessionWatcher) Stop() {
	close(e.stopCh)
}
