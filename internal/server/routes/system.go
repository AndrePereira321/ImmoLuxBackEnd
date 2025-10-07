package routes

type PingResponse struct {
	Status int `json:"status"`
}

func Ping(ctx *RouteContext) error {
	response := &PingResponse{
		Status: 0,
	}
	if ctx.Db().Ping() {
		response.Status = 1
	}
	return ctx.RespondData(response)
}
