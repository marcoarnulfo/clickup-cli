package tui

import "testing"

func TestNavGoToPushesParent(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).goTo(screenReport)
	if m.screen != screenReport {
		t.Fatalf("screen = %v, want screenReport", m.screen)
	}
	if len(m.nav) != 1 || m.nav[0] != screenHome {
		t.Errorf("nav = %v, want [home]", m.nav)
	}
}

func TestNavPopReturnsToParentAndEmpties(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).goTo(screenReport)
	m = m.pop()
	if m.screen != screenHome {
		t.Fatalf("screen = %v, want screenHome", m.screen)
	}
	if len(m.nav) != 0 {
		t.Errorf("nav = %v, want empty", m.nav)
	}
}

func TestNavPopOnEmptyChainIsNoOp(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome)
	m = m.pop()
	if m.screen != screenHome {
		t.Fatalf("screen = %v, want screenHome (pop on empty chain must be a no-op)", m.screen)
	}
	if len(m.nav) != 0 {
		t.Errorf("nav = %v, want empty", m.nav)
	}
}

func TestNavReplaceLeavesChainUntouched(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).goTo(screenReport)
	m = m.replace(screenLoading)
	if m.screen != screenLoading {
		t.Fatalf("screen = %v, want screenLoading", m.screen)
	}
	if len(m.nav) != 1 || m.nav[0] != screenHome {
		t.Errorf("nav = %v, want [home] unchanged by replace", m.nav)
	}
	m = m.pop()
	if m.screen != screenHome {
		t.Errorf("pop after replace -> screen = %v, want screenHome (parent still reachable)", m.screen)
	}
}

// TestNavGoToTruncatesInsteadOfDuplicating is what bounds the stack
// structurally, rather than relying on anyone remembering to clear it at the
// right moment.
func TestNavGoToTruncatesInsteadOfDuplicating(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).goTo(screenReport).goTo(screenLog)
	for range 5 {
		m = m.goTo(screenReport)
	}
	if m.screen != screenReport {
		t.Fatalf("screen = %v, want screenReport", m.screen)
	}
	if len(m.nav) != 1 || m.nav[0] != screenHome {
		t.Errorf("nav = %v, want [home] — the chain grew or kept a duplicate", m.nav)
	}
}
