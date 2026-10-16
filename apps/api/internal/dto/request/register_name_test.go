package request

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestRegisterNameLength(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"张三", true},
		{"ab", true},
		{strings.Repeat("张", 20), true},
		{"", false},
		{"张", false},
		{strings.Repeat("张", 21), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := RegisterReq{Name: tc.name, Email: "test@example.com", Password: "123456", Code: "123456"}
			err := binding.Validator.ValidateStruct(req)
			if (err == nil) != tc.valid {
				t.Fatalf("name %q: valid=%v, error=%v", tc.name, tc.valid, err)
			}
		})
	}
}
