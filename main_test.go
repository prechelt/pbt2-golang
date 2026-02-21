package main

import "testing"

func TestEvaluateAnswersTiePreferences(t *testing.T) {
	answers := map[int]string{}
	choices := append(append(append([]string{}, repeat("E", "I", 5)...), repeat("S", "N", 5)...), append(repeat("T", "F", 5), repeat("J", "P", 5)...)...)
	for i, c := range choices {
		answers[i] = c
	}
	result := evaluateAnswers(answers)
	if result == nil {
		t.Fatalf("expected TTT result")
	}
	if result.Type != "ISTJ" {
		t.Fatalf("expected ISTJ tie-break result, got %s", result.Type)
	}
}

func TestRcdFlowSendRejectAccept(t *testing.T) {
	a := newApp()
	a.users["alice"] = &User{Username: "alice", Profile: MemberProfile{Motto1: "m1", Country: "X"}}
	a.users["bob"] = &User{Username: "bob", Profile: MemberProfile{Motto1: "m2", Country: "X"}}

	if got := a.contactStatus("alice", "bob"); got != "no_contact" {
		t.Fatalf("initial status: %s", got)
	}

	a.setContact("alice", "bob", true)
	if got := a.contactStatus("alice", "bob"); got != "RCD_sent" {
		t.Fatalf("after send: %s", got)
	}

	a.setContact("alice", "bob", false)
	if got := a.contactStatus("alice", "bob"); got != "no_contact" {
		t.Fatalf("after reject: %s", got)
	}

	a.setContact("alice", "bob", true)
	a.setContact("bob", "alice", true)
	if got := a.contactStatus("alice", "bob"); got != "in_contact" {
		t.Fatalf("after accept: %s", got)
	}
}

func repeat(a, b string, count int) []string {
	out := make([]string, 0, count*2)
	for i := 0; i < count; i++ {
		out = append(out, a, b)
	}
	return out
}
