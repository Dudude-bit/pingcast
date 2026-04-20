package domain

// RequiresPro reports whether a Pro-gated feature should be blocked for
// the given plan. Returns true → the request must be rejected with 402.
func RequiresPro(p Plan) bool {
	return p != PlanPro
}
