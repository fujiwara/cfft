package cfft

// export for testing only
func (app *CFFT) Config() *Config {
	return app.config
}

func (app *CFFT) SetRunner(r FunctionRunner) {
	app.runner = r
}
