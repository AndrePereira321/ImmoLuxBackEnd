package routes

type LoginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Login(ctx *RouteContext) error {
	payload := &LoginPayload{}
	err := ctx.ReadBody(&payload)
	if err != nil {
		return err
	}

	return nil
}
