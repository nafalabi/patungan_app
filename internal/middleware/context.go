package middleware

import "github.com/labstack/echo/v4"

func GetString(c echo.Context, key string) string {
	val := c.Get(key)
	if val == nil {
		return ""
	}
	strVal, ok := val.(string)
	if !ok {
		return ""
	}
	return strVal
}

func GetUint(c echo.Context, key string) uint {
	val := c.Get(key)
	if val == nil {
		return 0
	}
	uintVal, ok := val.(uint)
	if !ok {
		return 0
	}
	return uintVal
}
