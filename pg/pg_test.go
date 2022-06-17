package pg

import (
	"testing"

	"github.com/bobg/pgtenant"
)

func TestTransform(t *testing.T) {
	pgtenant.TransformTester(t, "tenant_id", queries)
}
