package state

import (
	"sync"
	"time"
)

// UserState holds the FSM state for a user
type UserState struct {
	State     string
	Data      map[string]interface{}
	UpdatedAt time.Time
}

// AppState holds the application state
type AppState struct {
	mu         sync.RWMutex
	userStates map[int64]*UserState
	adminIDs   map[int64]bool
}

// NewAppState creates a new application state
func NewAppState() *AppState {
	return &AppState{
		userStates: make(map[int64]*UserState),
		adminIDs:   make(map[int64]bool),
	}
}

// GetUserState gets the current state for a user
func (s *AppState) GetUserState(userID int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if state, ok := s.userStates[userID]; ok {
		return state.State
	}
	return ""
}

// SetUserState sets the state for a user
func (s *AppState) SetUserState(userID int64, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, ok := s.userStates[userID]; !ok {
		s.userStates[userID] = &UserState{
			Data: make(map[string]interface{}),
		}
	}
	s.userStates[userID].State = state
	s.userStates[userID].UpdatedAt = time.Now()
}

// ClearUserState clears the state for a user
func (s *AppState) ClearUserState(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.userStates, userID)
}

// GetUserData gets data for a user
func (s *AppState) GetUserData(userID int64, key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if state, ok := s.userStates[userID]; ok {
		return state.Data[key]
	}
	return nil
}

// SetUserData sets data for a user
func (s *AppState) SetUserData(userID int64, key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, ok := s.userStates[userID]; !ok {
		s.userStates[userID] = &UserState{
			Data: make(map[string]interface{}),
		}
	}
	s.userStates[userID].Data[key] = value
	s.userStates[userID].UpdatedAt = time.Now()
}

// AddAdmin adds an admin ID
func (s *AppState) AddAdmin(adminID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.adminIDs[adminID] = true
}

// RemoveAdmin removes an admin ID
func (s *AppState) RemoveAdmin(adminID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.adminIDs, adminID)
}

// IsAdmin checks if a user is an admin
func (s *AppState) IsAdmin(adminID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.adminIDs[adminID]
}

// GetAllAdminIDs returns all admin IDs
func (s *AppState) GetAllAdminIDs() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	ids := make([]int64, 0, len(s.adminIDs))
	for id := range s.adminIDs {
		ids = append(ids, id)
	}
	return ids
}
