package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

//models
type User struct {
	ID string `json:"id"`
	Username string `json:"name"`
}

type Permissions struct {
	Allowed bool `json:"allowed"`
}

type ContextData struct {
	Summary string `json:"summary"`
}

//results 
type UserResult struct{
	User User
	Err error
}

type PermissionResult struct{
	Perm Permissions
	Err error
}

type ContextResult struct {
	Ctx ContextData
	Err error
}

func getUser(ctx context.Context) (User, error) {
	select {
	case <-time.After(10*time.Millisecond):
		return User{
			ID: "123",
			Username: "Name",
		}, nil
		case <-ctx.Done():
			return User{}, ctx.Err()
	}
}

func checkAccess(ctx context.Context) (Permissions, error) {
	select {
	case <-time.After(50*time.Millisecond):
		return Permissions {
			Allowed: true,
		}, nil
		case <-ctx.Done():
			return Permissions{}, ctx.Err()
	}
}

func getContext(ctx context.Context) (ContextData, error) {
	n := rand.Intn(3) //random behavior

	var delay time.Duration
	switch n {
	case 0:
		delay = 100* time.Millisecond
	case 1:
		delay = 3 * time.Second
	default:
		return ContextData{}, errors.New("vector memory failed")
	}

	select {
	case <-time.After(delay):
		return ContextData{Summary: "some chat info"}, nil
	case <-ctx.Done():
		return ContextData{}, ctx.Err()
	}
}

//handler
func chatSummaryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
	defer cancel()

	userCh := make(chan UserResult, 1)
	permCh := make(chan PermissionResult, 1)
	ctxCh := make(chan ContextResult, 1)

	var wg sync.WaitGroup
	wg.Add(3)

	//fan-out
	go func() {
		defer wg.Done()
		u, err := getUser(ctx)
		userCh <- UserResult{User: u, Err: err}
	}()

	go func() {
		defer wg.Done()
		p, err := checkAccess(ctx)
		permCh <- PermissionResult{Perm: p, Err: err}
	}()

	go func() {
		defer wg.Done()
		c, err := getContext(ctx)
		ctxCh <- ContextResult{Ctx: c, Err: err}
	}()

	var (
		user        User
		perm        Permissions
		contextData *ContextData
	)

	select {
	case res := <-userCh:
		if res.Err != nil {
			http.Error(w, "user service failed", http.StatusInternalServerError)
			return
		}
		user = res.User
	case <-ctx.Done():
		http.Error(w, "timeout on user service", http.StatusInternalServerError)
		return
	}

	select {
	case res := <-permCh:
		if res.Err != nil {
			http.Error(w, "permissions service failed", http.StatusInternalServerError)
			return
		}
		perm = res.Perm
	case <-ctx.Done():
		http.Error(w, "timeout on permissions service", http.StatusInternalServerError)
		return
	}

	select {
	case res := <-ctxCh:
		if res.Err == nil {
			contextData = &res.Ctx
		}
	case <-ctx.Done():
	}

	wg.Wait()

	type Response struct {
		User        User         `json:"user"`
		Permissions Permissions `json:"permissions"`
		Context     *ContextData `json:"context,omitempty"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(Response{
		User:        user,
		Permissions: perm,
		Context:     contextData,
	})
}


func main() {
	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/chat/summary", chatSummaryHandler)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}