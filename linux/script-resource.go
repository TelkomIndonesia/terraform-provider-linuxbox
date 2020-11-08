package linux

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform/communicator/remote"
	"github.com/spf13/cast"
)

const (
	attrScriptLifecycleCommands      = "lifecycle_commands"
	attrScriptLifecycleCommandCreate = "create"
	attrScriptLifecycleCommandRead   = "read"
	attrScriptLifecycleCommandUpdate = "update"
	attrScriptLifecycleCommandDelete = "delete"
	attrScriptTriggers               = "triggers"
	attrScriptEnvironment            = "environment"
	attrScriptSensitiveEnvironment   = "sensitive_environment"
	attrScriptInterpreter            = "interpreter"
	attrScriptWorkingDirectory       = "working_directory"
	attrScriptDirty                  = "dirty"
	attrScriptReadFailed             = "read_failed"
	attrScriptReadError              = "read_error"
	attrScriptOutput                 = "output"
)

var schemaScriptResource = map[string]*schema.Schema{
	attrScriptLifecycleCommands: {
		Type:     schema.TypeList,
		Required: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				attrScriptLifecycleCommandCreate: {
					Type:     schema.TypeString,
					Required: true,
				},
				attrScriptLifecycleCommandUpdate: {
					Type:     schema.TypeString,
					Optional: true,
				},
				attrScriptLifecycleCommandRead: {
					Type:     schema.TypeString,
					Required: true,
				},
				attrScriptLifecycleCommandDelete: {
					Type:     schema.TypeString,
					Required: true,
				},
			},
		},
	},
	attrScriptTriggers: {
		Type:     schema.TypeMap,
		Optional: true,
		ForceNew: true,
	},
	attrScriptEnvironment: {
		Type:     schema.TypeMap,
		Optional: true,
		Elem:     schema.TypeString,
	},
	attrScriptSensitiveEnvironment: {
		Type:      schema.TypeMap,
		Optional:  true,
		Elem:      schema.TypeString,
		Sensitive: true,
	},
	attrScriptInterpreter: {
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	},
	attrScriptWorkingDirectory: {
		Type:     schema.TypeString,
		Optional: true,
		Default:  ".",
	},

	attrScriptOutput: {
		Type:     schema.TypeString,
		Computed: true,
	},

	attrScriptDirty: {
		Type:     schema.TypeBool,
		Optional: true,
		Default:  false,
	},
	attrScriptReadFailed: {
		Type:     schema.TypeBool,
		Optional: true,
		Default:  false,
	},
	attrScriptReadError: {
		Type:     schema.TypeString,
		Optional: true,
		Default:  "",
	},
}

type handlerScriptResource struct{}

func (h handlerScriptResource) newScript(rd *schema.ResourceData, l *linux, attrLifeCycle string) (s *script) {
	if rd == nil {
		return
	}

	lc := cast.ToSlice(rd.Get(attrScriptLifecycleCommands))[0]
	s = &script{
		l: l,

		workdir:     cast.ToString(rd.Get(attrScriptWorkingDirectory)),
		env:         cast.ToStringMapString(rd.Get(attrScriptEnvironment)),
		interpreter: cast.ToStringSlice(rd.Get(attrScriptInterpreter)),
		body:        cast.ToStringMapString(lc)[attrLifeCycle],
	}
	for k, v := range cast.ToStringMapString(rd.Get(attrScriptSensitiveEnvironment)) {
		s.env[k] = v
	}
	return
}

func (h handlerScriptResource) read(ctx context.Context, rd *schema.ResourceData, l *linux) (err error) {
	sc := h.newScript(rd, l, attrScriptLifecycleCommandRead)
	res, err := sc.exec(ctx)
	if err != nil {
		return
	}
	if err = rd.Set(attrScriptOutput, res); err != nil {
		return
	}
	return
}

func (h handlerScriptResource) Read(ctx context.Context, rd *schema.ResourceData, meta interface{}) (d diag.Diagnostics) {
	old := cast.ToString(rd.Get(attrScriptOutput))

	err := h.read(ctx, rd, meta.(*linux))
	var errExit *remote.ExitError
	switch {
	case errors.As(err, &errExit):
		_ = rd.Set(attrScriptReadFailed, true)
		_ = rd.Set(attrScriptReadError, err.Error())
		return

	default:
		_ = rd.Set(attrScriptReadFailed, false)
		_ = rd.Set(attrScriptReadError, "")
	}
	if err != nil {
		return diag.FromErr(err)
	}

	new := cast.ToString(rd.Get(attrScriptOutput))
	if err := rd.Set(attrScriptDirty, old != new); err != nil {
		return diag.FromErr(err)
	}
	return
}

func (h handlerScriptResource) Create(ctx context.Context, rd *schema.ResourceData, meta interface{}) (d diag.Diagnostics) {
	l := meta.(*linux)
	sc := h.newScript(rd, l, attrScriptLifecycleCommandCreate)
	if _, err := sc.exec(ctx); err != nil {
		return diag.FromErr(err)
	}

	if err := h.read(ctx, rd, l); err != nil {
		return diag.FromErr(err)
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return diag.FromErr(err)
	}
	rd.SetId(id.String())
	return
}

// WARN: see https://github.com/hashicorp/terraform-plugin-sdk/issues/476
func (h handlerScriptResource) restoreOldResourceData(rd *schema.ResourceData, except ...string) (err error) {
	var exceptMap = map[string]bool{}
	for _, k := range except {
		exceptMap[k] = true
	}
	toRestore := []string{
		attrScriptLifecycleCommands,
		attrScriptEnvironment,
		attrScriptSensitiveEnvironment,
		attrScriptInterpreter,
		attrScriptWorkingDirectory,
		attrScriptOutput,
		attrScriptDirty,
		attrScriptReadFailed,
		attrScriptReadError,
	}

	for _, k := range toRestore {
		if exceptMap[k] {
			continue
		}
		o, _ := rd.GetChange(k)
		err = rd.Set(k, o)
		if err != nil {
			return
		}
	}
	return
}

func (h handlerScriptResource) Update(ctx context.Context, rd *schema.ResourceData, meta interface{}) (d diag.Diagnostics) {
	if rd.HasChange(attrScriptLifecycleCommands) {
		// mimic the behaviour when terraform provider is updated,
		// that is no old logic are executed and
		// the new logic are run with the existing state and diff
		_ = h.restoreOldResourceData(rd, attrScriptLifecycleCommands)
		return
	}

	l := meta.(*linux)
	sc := h.newScript(rd, l, attrScriptLifecycleCommandUpdate)
	oldOutput := cast.ToString(rd.Get(attrScriptOutput))
	sc.stdin = strings.NewReader(oldOutput)
	if _, err := sc.exec(ctx); err != nil {
		_ = h.restoreOldResourceData(rd)
		return diag.FromErr(err)
	}

	if err := h.read(ctx, rd, l); err != nil {
		return diag.FromErr(err)
	}
	return
}

func (h handlerScriptResource) Delete(ctx context.Context, rd *schema.ResourceData, meta interface{}) (d diag.Diagnostics) {
	l := meta.(*linux)
	sc := h.newScript(rd, l, attrScriptLifecycleCommandDelete)
	if _, err := sc.exec(ctx); err != nil {
		return diag.FromErr(err)
	}
	return
}

func (h handlerScriptResource) CustomizeDiff(c context.Context, rd *schema.ResourceDiff, meta interface{}) (err error) {
	if rd.Id() == "" {
		return // no state
	}

	if rd.HasChange(attrScriptLifecycleCommands) {
		var forbidden []string
		for _, key := range rd.GetChangedKeysPrefix("") {
			if !strings.HasPrefix(key, attrScriptLifecycleCommands) &&
				!strings.HasPrefix(key, attrScriptReadError) &&
				!strings.HasPrefix(key, attrScriptReadFailed) {

				forbidden = append(forbidden, key)
			}
		}
		if len(forbidden) > 0 {
			return fmt.Errorf("update to `%s` should not be combined with update to other arguments: %s", attrScriptLifecycleCommands, strings.Join(forbidden, ","))
		}
		return // updated commands. let Update handle it.
	}
	if f, _ := rd.GetChange(attrScriptReadFailed); cast.ToBool(f) {
		_ = rd.ForceNew(attrScriptReadFailed) // read failed but no update in commands, force recreation
		return
	}

	if _, ok := rd.GetOk(attrScriptLifecycleCommands + ".0." + attrScriptLifecycleCommandUpdate); ok {
		return // updateable
	}

	for _, key := range rd.GetChangedKeysPrefix("") {
		if strings.HasPrefix(key, attrScriptTriggers) {
			continue // already force new.
		}

		// need to remove index from map and list
		switch {
		case strings.HasPrefix(key, attrScriptEnvironment):
			fallthrough
		case strings.HasPrefix(key, attrScriptSensitiveEnvironment):
			fallthrough
		case strings.HasPrefix(key, attrScriptInterpreter):
			parts := strings.Split(key, ".")
			key = strings.Join(parts[:len(parts)-1], ".")
		}
		err = rd.ForceNew(key)
		if err != nil {
			return
		}
	}
	return
}

func scriptResource() *schema.Resource {
	var h handlerScriptResource
	return &schema.Resource{
		Schema:        schemaScriptResource,
		CreateContext: h.Create,
		ReadContext:   h.Read,
		UpdateContext: h.Update,
		DeleteContext: h.Delete,
		CustomizeDiff: h.CustomizeDiff,
	}
}
