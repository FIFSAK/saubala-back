package user

import "testing"

func TestRolePredicates(t *testing.T) {
	cases := []struct {
		role               Role
		valid, admin, assn bool
	}{
		{RoleSuperAdmin, true, true, false},
		{RoleAdmin, true, true, true},
		{RoleUser, true, false, true},
		{Role("ghost"), false, false, false},
	}
	for _, c := range cases {
		if got := c.role.IsValid(); got != c.valid {
			t.Errorf("%s.IsValid() = %v, want %v", c.role, got, c.valid)
		}
		if got := c.role.IsAdmin(); got != c.admin {
			t.Errorf("%s.IsAdmin() = %v, want %v", c.role, got, c.admin)
		}
		if got := c.role.IsAssignable(); got != c.assn {
			t.Errorf("%s.IsAssignable() = %v, want %v", c.role, got, c.assn)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	valid := []string{"a@b.kz", "user.name@example.com"}
	invalid := []string{
		"", "  ", "not-an-email", "a@", "@b.com",
		"Admin <admin@x.com>", `"x" <a@b.com>`, "a@b.com (c)",
	}
	for _, e := range valid {
		if err := ValidateEmail(e); err != nil {
			t.Errorf("ValidateEmail(%q) unexpected error: %v", e, err)
		}
	}
	for _, e := range invalid {
		if err := ValidateEmail(e); err == nil {
			t.Errorf("ValidateEmail(%q) expected error, got nil", e)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("1234567"); err == nil {
		t.Error("expected error for 7-char password")
	}
	if err := ValidatePassword("12345678"); err != nil {
		t.Errorf("unexpected error for 8-char password: %v", err)
	}
}

func TestNew(t *testing.T) {
	u, err := New("  Admin@Example.COM ", "Jane", RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == "" {
		t.Error("expected generated id")
	}
	if u.Email != "admin@example.com" {
		t.Errorf("email not normalized: %q", u.Email)
	}
	if !u.IsActive {
		t.Error("new user should be active")
	}
	if u.CreatedAt.IsZero() || u.UpdatedAt.IsZero() {
		t.Error("timestamps not set")
	}

	if _, err := New("bad", "x", RoleUser); err == nil {
		t.Error("expected error for invalid email")
	}
	if _, err := New("ok@ok.kz", "x", Role("nope")); err == nil {
		t.Error("expected error for invalid role")
	}
}
