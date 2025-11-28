package lcel

import (
	"context"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	StringField             ResolvedValue[string]
	BoolField               ResolvedValue[bool]
	DynTypeField            ResolvedValue[any]
	StaticStringField       ResolvedValue[string]
	StaticBoolField         ResolvedValue[bool]
	StaticDynTypeFieldField ResolvedValue[any]
	StringFieldPtr          *ResolvedValue[string]
	BoolFieldPtr            *ResolvedValue[bool]

	Unresolved *ResolvedValue[string]
}

func TestBind(t *testing.T) {
	tc := testCase{}

	variables := []cel.EnvOption{
		cel.Variable("string_field", cel.StringType),
		cel.Variable("bool_field", cel.BoolType),
		cel.Variable("dynamic_field", cel.DynType),
	}

	require.NoError(t, Bind("${string_field} ${string_field}", &tc.StringField, variables...))
	require.NoError(t, Bind("${bool_field}", &tc.BoolField, variables...))
	require.NoError(t, Bind("${dynamic_field.other_key}", &tc.DynTypeField, variables...))

	require.NoError(t, BindPtr("${string_field}", &tc.StringFieldPtr, variables...))
	require.NoError(t, BindPtr("${bool_field}", &tc.BoolFieldPtr, variables...))

	require.NoError(t, Bind("some value", &tc.StaticStringField, variables...))
	require.NoError(t, Bind("true", &tc.StaticBoolField, variables...))
	require.NoError(t, Bind("{\"key\": \"value\"}", &tc.StaticDynTypeFieldField, variables...))

	getValue := func(v any, err error) any {
		require.NoError(t, err)
		return v
	}

	contextData := map[string]any{
		"string_field": "some value",
		"bool_field":   false,
		"dynamic_field": map[string]any{
			"other_key": "other value",
		},
	}
	ctx := context.Background()

	assert.Equal(t, "some value some value", getValue(tc.StringField.Resolve(ctx, contextData)))
	assert.Equal(t, false, getValue(tc.BoolField.Resolve(ctx, contextData)))
	assert.Equal(t, "other value", getValue(tc.DynTypeField.Resolve(ctx, contextData)))

	assert.Equal(t, "some value", getValue(tc.StaticStringField.Resolve(ctx, contextData)))
	assert.Equal(t, true, getValue(tc.StaticBoolField.Resolve(ctx, contextData)))
	assert.Equal(t, map[string]any{
		"key": "value",
	}, getValue(tc.StaticDynTypeFieldField.Resolve(ctx, contextData)))

	assert.Equal(t, "some value", getValue(tc.StringFieldPtr.Resolve(ctx, contextData)))
	assert.Equal(t, false, getValue(tc.BoolFieldPtr.Resolve(ctx, contextData)))

	_, err := tc.Unresolved.Resolve(ctx, contextData)
	assert.Error(t, err)

}
