package cfft

// export for testing only
func (app *CFFT) Config() *Config {
	return app.config
}

func (app *CFFT) SetRunner(r FunctionRunner) {
	app.runner = r
}

func (tc *TestCase) GetEvent() *CFFEvent {
	return tc.event
}

func (tc *TestCase) GetExpect() *CFFExpect {
	return tc.expect
}
