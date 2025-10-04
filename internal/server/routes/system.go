package routes

func Ping(ctx *RouteContext) error {
	status := 0
	if ctx.DbPing() {
		status = 1
	}
	return ctx.Respond(map[string]int{"status": status})
}
