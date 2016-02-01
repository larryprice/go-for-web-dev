package models_test

import (
  "testing"
  . "./"
)

func TestAuthenticateFailsWithIncorrectPassword(t *testing.T) {
  user := NewUser("gopher@golang.org", "password")
  if user.Authenticate("wrong") {
    t.Errorf("Authenticate should return false for incorrect password")
  }
}

func TestAuthenticateSucceedsWithCorrectPassword(t *testing.T) {
  user := NewUser("gopher@golang.org", "password")
  if !user.Authenticate("password") {
    t.Errorf("Authenticate should return true for correct password")
  }
}
