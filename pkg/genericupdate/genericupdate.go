// Package genericupdate implements Python az's generic update path syntax
// (--set, --add, --remove) against a map[string]interface{} resource body.
package genericupdate

import (
  "encoding/json"
  "fmt"
  "regexp"
  "strconv"
  "strings"
)

type OpKind int

const (
  Set OpKind = iota
  Add
  Remove
)

type Op struct {
  Kind  OpKind
  Path  string
  Value string // raw value as supplied on CLI; parsed per Kind
}

// Apply mutates obj per the slice of operations, in order.
func Apply(obj map[string]interface{}, ops []Op) error {
  for _, op := range ops {
    switch op.Kind {
    case Set:
      if err := applySet(obj, op.Path, op.Value); err != nil {
        return fmt.Errorf("--set %s: %w", op.Path, err)
      }
    case Add:
      if err := applyAdd(obj, op.Path, op.Value); err != nil {
        return fmt.Errorf("--add %s: %w", op.Path, err)
      }
    case Remove:
      if err := applyRemove(obj, op.Path, op.Value); err != nil {
        return fmt.Errorf("--remove %s: %w", op.Path, err)
      }
    }
  }
  return nil
}

// path = key("." key | "[" index "]")*
var indexRE = regexp.MustCompile(`^\[(\d+)\]`)

type segment struct {
  key     string // map key, empty if isIndex
  index   int    // list index, -1 if not isIndex
  isIndex bool
}

func parsePath(path string) ([]segment, error) {
  if path == "" {
    return nil, fmt.Errorf("empty path")
  }
  segs := []segment{}
  for path != "" {
    if m := indexRE.FindStringSubmatch(path); m != nil {
      n, _ := strconv.Atoi(m[1])
      segs = append(segs, segment{index: n, isIndex: true})
      path = path[len(m[0]):]
      if strings.HasPrefix(path, ".") {
        path = path[1:]
      }
      continue
    }
    dot := strings.IndexAny(path, ".[")
    if dot == -1 {
      segs = append(segs, segment{key: path})
      path = ""
    } else {
      segs = append(segs, segment{key: path[:dot]})
      if path[dot] == '.' {
        path = path[dot+1:]
      } else {
        path = path[dot:]
      }
    }
  }
  return segs, nil
}

// parseValue tries to JSON-unmarshal value; falls back to a plain string.
func parseValue(value string) interface{} {
  var v interface{}
  if err := json.Unmarshal([]byte(value), &v); err == nil {
    return v
  }
  return value
}

func applySet(obj map[string]interface{}, path, value string) error {
  segs, err := parsePath(path)
  if err != nil {
    return err
  }
  parsed := parseValue(value)
  return setAtPath(obj, segs, parsed)
}

func setAtPath(root map[string]interface{}, segs []segment, value interface{}) error {
  if len(segs) == 0 {
    return fmt.Errorf("empty path")
  }
  // Navigate to parent of final segment, creating maps along the way.
  var cursor interface{} = root
  for i := 0; i < len(segs)-1; i++ {
    seg := segs[i]
    next := segs[i+1]
    if seg.isIndex {
      list, ok := cursor.([]interface{})
      if !ok {
        return fmt.Errorf("expected list at index %d", seg.index)
      }
      if seg.index < 0 || seg.index >= len(list) {
        return fmt.Errorf("index %d out of range", seg.index)
      }
      cursor = list[seg.index]
      continue
    }
    m, ok := cursor.(map[string]interface{})
    if !ok {
      return fmt.Errorf("expected map at key %q", seg.key)
    }
    if _, exists := m[seg.key]; !exists {
      // Auto-create intermediate map; if next is index we'd need a list,
      // but auto-creating lists is unsupported (matches Python behavior).
      if next.isIndex {
        return fmt.Errorf("cannot auto-create list at %q", seg.key)
      }
      m[seg.key] = map[string]interface{}{}
    }
    cursor = m[seg.key]
  }
  last := segs[len(segs)-1]
  if last.isIndex {
    list, ok := cursor.([]interface{})
    if !ok {
      return fmt.Errorf("expected list for index %d", last.index)
    }
    if last.index < 0 || last.index >= len(list) {
      return fmt.Errorf("index %d out of range", last.index)
    }
    list[last.index] = value
    return nil
  }
  m, ok := cursor.(map[string]interface{})
  if !ok {
    return fmt.Errorf("expected map for key %q", last.key)
  }
  m[last.key] = value
  return nil
}

func applyAdd(obj map[string]interface{}, path, value string) error {
  segs, err := parsePath(path)
  if err != nil {
    return err
  }
  parent, last, err := navigateToParent(obj, segs)
  if err != nil {
    return err
  }
  // last must point at a list inside parent.
  if last.isIndex {
    return fmt.Errorf("--add path must end at a map key, not an index")
  }
  m, ok := parent.(map[string]interface{})
  if !ok {
    return fmt.Errorf("expected map at parent of %q", last.key)
  }
  cur, ok := m[last.key].([]interface{})
  if !ok {
    return fmt.Errorf("path %q does not refer to a list", last.key)
  }
  m[last.key] = append(cur, parseValue(value))
  return nil
}

func applyRemove(obj map[string]interface{}, path, value string) error {
  segs, err := parsePath(path)
  if err != nil {
    return err
  }
  parent, last, err := navigateToParent(obj, segs)
  if err != nil {
    return err
  }
  if last.isIndex {
    return fmt.Errorf("--remove path must end at a key (use 'list_path INDEX' to remove an element)")
  }
  m, ok := parent.(map[string]interface{})
  if !ok {
    return fmt.Errorf("expected map at parent of %q", last.key)
  }
  // If value is a numeric index, treat path as a list and remove that index.
  if value != "" {
    idx, err := strconv.Atoi(value)
    if err != nil {
      return fmt.Errorf("--remove value must be a list index, got %q", value)
    }
    list, ok := m[last.key].([]interface{})
    if !ok {
      return fmt.Errorf("path %q does not refer to a list", last.key)
    }
    if idx < 0 || idx >= len(list) {
      return fmt.Errorf("index %d out of range", idx)
    }
    m[last.key] = append(list[:idx], list[idx+1:]...)
    return nil
  }
  delete(m, last.key)
  return nil
}

// navigateToParent walks segs[:-1] and returns the parent value plus the last segment.
func navigateToParent(root map[string]interface{}, segs []segment) (interface{}, segment, error) {
  if len(segs) == 0 {
    return nil, segment{}, fmt.Errorf("empty path")
  }
  var cursor interface{} = root
  for i := 0; i < len(segs)-1; i++ {
    seg := segs[i]
    if seg.isIndex {
      list, ok := cursor.([]interface{})
      if !ok {
        return nil, segment{}, fmt.Errorf("expected list at index %d", seg.index)
      }
      if seg.index < 0 || seg.index >= len(list) {
        return nil, segment{}, fmt.Errorf("index %d out of range", seg.index)
      }
      cursor = list[seg.index]
      continue
    }
    m, ok := cursor.(map[string]interface{})
    if !ok {
      return nil, segment{}, fmt.Errorf("expected map at key %q", seg.key)
    }
    cursor = m[seg.key]
  }
  return cursor, segs[len(segs)-1], nil
}
