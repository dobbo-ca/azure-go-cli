package genericupdate

import (
  "encoding/json"
  "reflect"
  "testing"
)

func mustJSON(s string) map[string]interface{} {
  var m map[string]interface{}
  if err := json.Unmarshal([]byte(s), &m); err != nil {
    panic(err)
  }
  return m
}

func TestApplySet(t *testing.T) {
  t.Run("set top-level string", func(t *testing.T) {
    obj := mustJSON(`{"location":"eastus","tags":{}}`)
    err := Apply(obj, []Op{{Kind: Set, Path: "location", Value: "westus"}})
    if err != nil {
      t.Fatal(err)
    }
    if obj["location"] != "westus" {
      t.Errorf("got %v", obj["location"])
    }
  })

  t.Run("set nested path creates intermediate maps", func(t *testing.T) {
    obj := mustJSON(`{}`)
    err := Apply(obj, []Op{{Kind: Set, Path: "tags.env", Value: "prod"}})
    if err != nil {
      t.Fatal(err)
    }
    want := mustJSON(`{"tags":{"env":"prod"}}`)
    if !reflect.DeepEqual(obj, want) {
      t.Errorf("got %v want %v", obj, want)
    }
  })

  t.Run("set with JSON value", func(t *testing.T) {
    obj := mustJSON(`{}`)
    err := Apply(obj, []Op{{Kind: Set, Path: "properties.networkAcls", Value: `{"defaultAction":"Deny"}`}})
    if err != nil {
      t.Fatal(err)
    }
    nacl := obj["properties"].(map[string]interface{})["networkAcls"].(map[string]interface{})
    if nacl["defaultAction"] != "Deny" {
      t.Errorf("got %v", nacl)
    }
  })

  t.Run("set list element by index", func(t *testing.T) {
    obj := mustJSON(`{"properties":{"subnets":[{"name":"a"},{"name":"b"}]}}`)
    err := Apply(obj, []Op{{Kind: Set, Path: "properties.subnets[1].name", Value: "renamed"}})
    if err != nil {
      t.Fatal(err)
    }
    got := obj["properties"].(map[string]interface{})["subnets"].([]interface{})[1].(map[string]interface{})["name"]
    if got != "renamed" {
      t.Errorf("got %v", got)
    }
  })
}
