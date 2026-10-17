package cachetapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// input holds a request's parameters the way Cachet reads them: the query
// string, then a JSON or form body on top.
type input map[string]any

func readInput(c *gin.Context) (input, error) {
	in := input{}
	for k, v := range c.Request.URL.Query() {
		in.setForm(k, v)
	}
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return in, nil
	}
	ct := c.ContentType()
	switch {
	case strings.Contains(ct, "json"):
		var body map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("invalid JSON body: %w", err)
		}
		for k, v := range body {
			in[k] = v
		}
	case strings.Contains(ct, "form"):
		if err := c.Request.ParseMultipartForm(1 << 20); err != nil && c.Request.PostForm == nil {
			if err := c.Request.ParseForm(); err != nil {
				return nil, fmt.Errorf("invalid form body: %w", err)
			}
		}
		for k, v := range c.Request.PostForm {
			in.setForm(k, v)
		}
	}
	return in, nil
}

// setForm stores form values, folding PHP-style keys: tags[]=a&tags[]=b is
// a list, and meta[key]=value or components[3]=1 a map.
func (in input) setForm(key string, values []string) {
	if strings.HasSuffix(key, "[]") {
		in[strings.TrimSuffix(key, "[]")] = toAny(values)
		return
	}
	if i := strings.Index(key, "["); i > 0 && strings.HasSuffix(key, "]") {
		name, sub := key[:i], key[i+1:len(key)-1]
		m, _ := in[name].(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		m[sub] = values[len(values)-1]
		in[name] = m
		return
	}
	in[key] = values[len(values)-1]
}

func toAny(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

func (in input) has(key string) bool {
	v, ok := in[key]
	return ok && v != nil
}

func (in input) str(key string) (string, bool) {
	v, ok := in[key]
	if !ok || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(t), true
	}
	return fmt.Sprint(v), true
}

func (in input) integer(key string) (int, bool, error) {
	s, ok := in.str(key)
	if !ok || strings.TrimSpace(s) == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, true, fmt.Errorf("%s must be an integer", key)
	}
	return n, true, nil
}

// boolean reads true/false, 1/0, on/off and yes/no, as PHP does.
func (in input) boolean(key string) (bool, bool, error) {
	s, ok := in.str(key)
	if !ok {
		return false, false, nil
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "on", "yes":
		return true, true, nil
	case "0", "false", "off", "no", "":
		return false, true, nil
	}
	return false, true, fmt.Errorf("%s must be a boolean", key)
}

// stringMap reads an object of strings, such as meta.
func (in input) stringMap(key string) (map[string]string, bool) {
	m, ok := in[key].(map[string]any)
	if !ok {
		return nil, false
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = fmt.Sprint(v)
	}
	return out, true
}

// list reads a comma-separated string or an array, such as tags.
func (in input) list(key string) ([]string, bool) {
	switch v := in[key].(type) {
	case string:
		out := []string{}
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out, true
	case []any:
		out := []string{}
		for _, p := range v {
			if s := strings.TrimSpace(fmt.Sprint(p)); s != "" {
				out = append(out, s)
			}
		}
		return out, true
	}
	return nil, false
}

// idMap reads Cachet's schedule components: an object from component id to
// status, or a plain list of ids. It returns the ids.
func (in input) idMap(key string) ([]uint, bool, error) {
	var keys []string
	switch v := in[key].(type) {
	case map[string]any:
		for k := range v {
			keys = append(keys, k)
		}
	case []any:
		for _, p := range v {
			keys = append(keys, fmt.Sprint(p))
		}
	case nil:
		return nil, false, nil
	default:
		return nil, true, fmt.Errorf("%s must be an object of component ids", key)
	}
	ids := make([]uint, 0, len(keys))
	for _, k := range keys {
		n, err := strconv.ParseUint(strings.TrimSpace(k), 10, 32)
		if err != nil {
			return nil, true, fmt.Errorf("%s: invalid component id %q", key, k)
		}
		ids = append(ids, uint(n))
	}
	return ids, true, nil
}
