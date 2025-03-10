// Copyright 2018 The Nakama Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
	"sort"
)

const codeTemplate string = `-- Code generated by codegen/main.go. DO NOT EDIT.

--[[--
The Nakama client SDK for Defold.

@module nakama
]]

local json = require "nakama.util.json"
local b64 = require "nakama.util.b64"
local log = require "nakama.util.log"
local async = require "nakama.util.async"
local retries = require "nakama.util.retries"
local api_session = require "nakama.session"
local socket = require "nakama.socket"

local uri = require "nakama.util.uri"
local uri_encode = uri.encode

local M = {}

--
-- Defines
--

{{- range $defname, $definition := .Definitions }}
{{- $classname := $defname | title }}
{{- if $definition.Enum }}

--- {{ $classname | pascalToSnake }}
-- {{ $definition.Description | stripNewlines }}
{{- range $i, $enum := $definition.Enum }}
M.{{ $classname | uppercase }}_{{ $enum }} = "{{ $enum }}"
{{- end }}
{{- end }}
{{- end }}

--
-- The low level client for the Nakama API.
--

local _config = {}


--- Create a Nakama client instance.
-- @param config A table of configuration options.
-- config.engine - Engine specific implementations.
-- config.host
-- config.port
-- config.timeout
-- config.use_ssl - Use secure or non-secure sockets.
-- config.bearer_token
-- config.username
-- config.password
-- @return Nakama Client instance.
function M.create_client(config)
	assert(config, "You must provide a configuration")
	assert(config.host, "You must provide a host")
	assert(config.port, "You must provide a port")
	assert(config.engine, "You must provide an engine")
	assert(type(config.engine.http) == "function", "The engine must provide the 'http' function")
	assert(type(config.engine.socket_create) == "function", "The engine must provide the 'socket_create' function")
	assert(type(config.engine.socket_connect) == "function", "The engine must provide the 'socket_connect' function")
	assert(type(config.engine.socket_send) == "function", "The engine must provide the 'socket_send' function")
	log("init()")

	local client = {}
	local scheme = config.use_ssl and "https" or "http"
	client.engine = config.engine
	client.config = {}
	client.config.host = config.host
	client.config.port = config.port
	client.config.http_uri = ("%s://%s:%d"):format(scheme, config.host, config.port)
	client.config.bearer_token = config.bearer_token
	client.config.username = config.username
	client.config.password = config.password
	client.config.timeout = config.timeout or 10
	client.config.use_ssl = config.use_ssl
	client.config.retry_policy = config.retry_policy or retries.none()

	local ignored_fns = { create_client = true, sync = true }
	for name,fn in pairs(M) do
		if not ignored_fns[name] and type(fn) == "function" then
			log("setting " .. name)
			client[name] = function(...) return fn(client, ...) end
		end
	end

	return client
end


--- Create a Nakama socket.
-- @param client The client to create the socket for.
-- @return Socket instance.
function M.create_socket(client)
	assert(client, "You must provide a client")
	return socket.create(client)
end

--- Set Nakama client bearer token.
-- @param client Nakama client.
-- @param bearer_token Authorization bearer token.
function M.set_bearer_token(client, bearer_token)
	assert(client, "You must provide a client")
	client.config.bearer_token = bearer_token
end


-- cancellation tokens associated with a coroutine
local cancellation_tokens = {}

-- cancel a cancellation token
function M.cancel(token)
	assert(token)
	token.cancelled = true
end

-- create a cancellation token
-- use this to cancel an ongoing API call or a sequence of API calls
-- @return token Pass the token to a call to nakama.sync() or to any of the API calls
function M.cancellation_token()
	local token = {
		cancelled = false
	}
	function token.cancel()
		token.cancelled = true
	end
	return token
end

-- Private
-- Run code within a coroutine
-- @param fn The code to run
-- @param cancellation_token Optional cancellation token to cancel the running code
function M.sync(fn, cancellation_token)
	assert(fn)
	local co = nil
	co = coroutine.create(function()
		cancellation_tokens[co] = cancellation_token
		fn()
		cancellation_tokens[co] = nil
	end)
	local ok, err = coroutine.resume(co)
	if not ok then
		log(err)
		cancellation_tokens[co] = nil
	end
end

--
-- Nakama REST API
--

-- http request helper used to reduce code duplication in all API functions below
local function http(client, callback, url_path, query_params, method, post_data, retry_policy, cancellation_token, handler_fn)
	if callback then
		log(url_path, "with callback")
		client.engine.http(client.config, url_path, query_params, method, post_data, retry_policy, cancellation_token, function(result)
			if not cancellation_token or not cancellation_token.cancelled then
				callback(handler_fn(result))
			end
		end)
	else
		log(url_path, "with coroutine")
		local co = coroutine.running()
		assert(co, "You must be running this from withing a coroutine")

		-- get cancellation token associated with this coroutine
		cancellation_token = cancellation_tokens[co]
		if cancellation_token and cancellation_token.cancelled then
			cancellation_tokens[co] = nil
			return
		end

		return async(function(done)
			client.engine.http(client.config, url_path, query_params, method, post_data, retry_policy, cancellation_token, function(result)
				if cancellation_token and cancellation_token.cancelled then
					cancellation_tokens[co] = nil
					return
				end
				done(handler_fn(result))
			end)
		end)
	end
end


{{- range $url, $path := .Paths }}
	{{- range $method, $operation := $path}}

--- {{ $operation.OperationId | pascalToSnake | removePrefix }}
-- {{ $operation.Summary | stripNewlines }}
-- @param client Nakama client.
{{- range $i, $parameter := $operation.Parameters }}
{{- $luaType := luaType $parameter.Type $parameter.Schema.Ref }}
{{- $varName := varName $parameter.Name $parameter.Type $parameter.Schema.Ref }}
{{- $varName := $varName | pascalToSnake }}
{{- $varComment := varComment $parameter.Name $parameter.Type $parameter.Schema.Ref $parameter.Items.Type }}
{{- if and (eq $parameter.In "body") $parameter.Schema.Ref }}
{{- bodyFunctionArgsDocs $parameter.Schema.Ref }}
{{- end }}
{{- if and (eq $parameter.In "body") $parameter.Schema.Type }}
-- @param body ({{ $parameter.Schema.Type }}) {{ $parameter.Description | stripNewlines }}
{{- end }}
{{- if ne $parameter.In "body" }}
-- @param {{ $varName }} ({{ $parameter.Schema.Type }}) {{ $parameter.Description | stripNewlines }}
{{- end }}

{{- end }}
-- @param callback Optional callback function
-- A coroutine is used and the result is returned if no callback function is provided.
-- @param retry_policy Optional retry policy used specifically for this call or nil
-- @param cancellation_token Optional cancellation token for this call
-- @return The result.
function M.{{ $operation.OperationId | pascalToSnake | removePrefix }}(client
	{{- range $i, $parameter := $operation.Parameters }}
	{{- $luaType := luaType $parameter.Type $parameter.Schema.Ref }}
	{{- $varName := varName $parameter.Name $parameter.Type $parameter.Schema.Ref }}
	{{- $varName := $varName | pascalToSnake }}
	{{- $varComment := varComment $parameter.Name $parameter.Type $parameter.Schema.Ref $parameter.Items.Type }}
	{{- if and (eq $parameter.In "body") $parameter.Schema.Ref }}
	{{- bodyFunctionArgs $parameter.Schema.Ref}}
	{{- end }}
	{{- if and (eq $parameter.In "body") $parameter.Schema.Type }}, {{ $parameter.Name }} {{- end }}
	{{- if ne $parameter.In "body" }}, {{ $varName }} {{- end }}
	{{- end }}, callback, retry_policy, cancellation_token)
	assert(client, "You must provide a client")
	{{- range $parameter := $operation.Parameters }}
	{{- $varName := varName $parameter.Name $parameter.Type $parameter.Schema.Ref }}
	{{- if eq $parameter.In "body" }}
	{{- bodyFunctionArgsAssert $parameter.Schema.Ref}}
	{{- end }}
	{{- if and (eq $parameter.In "body") $parameter.Schema.Type }}
	assert({{- if $parameter.Required }}body and {{ end }}type(body) == "{{ $parameter.Schema.Type }}", "Argument 'body' must be of type '{{ $parameter.Schema.Type }}'")
	{{- end }}

	{{- end }}

	{{- if $operation.OperationId | isAuthenticateMethod }}
	-- unset the token so username+password credentials will be used
	client.config.bearer_token = nil

	{{- end}}

	local url_path = "{{- $url }}"
	{{- range $parameter := $operation.Parameters }}
	{{- $varName := varName $parameter.Name $parameter.Type $parameter.Schema.Ref }}
	{{- if eq $parameter.In "path" }}
	url_path = url_path:gsub("{{- print "{" $parameter.Name "}"}}", uri_encode({{ $varName | pascalToSnake }}))
	{{- end }}
	{{- end }}

	local query_params = {}
	{{- range $parameter := $operation.Parameters}}
	{{- $varName := varName $parameter.Name $parameter.Type $parameter.Schema.Ref }}
	{{- if eq $parameter.In "query"}}
	query_params["{{- $parameter.Name }}"] = {{ $varName | pascalToSnake }}
	{{- end}}
	{{- end}}

	local post_data = nil
	{{- range $parameter := $operation.Parameters }}
	{{- $varName := varName $parameter.Name $parameter.Type $parameter.Schema.Ref }}
	{{- if eq $parameter.In "body" }}
	{{- if $parameter.Schema.Ref }}
	post_data = json.encode({
		{{- bodyFunctionArgsTable $parameter.Schema.Ref}}	})
	{{- end }}
	{{- if $parameter.Schema.Type }}
	post_data = json.encode(body)
	{{- end }}
		{{- end }}
	{{- end }}

	return http(client, callback, url_path, query_params, "{{- $method | uppercase }}", post_data, retry_policy, cancellation_token, function(result)
		{{- if $operation.Responses.Ok.Schema.Ref }}
		if not result.error and {{ $operation.Responses.Ok.Schema.Ref | cleanRef | pascalToSnake }} then
			result = {{ $operation.Responses.Ok.Schema.Ref | cleanRef | pascalToSnake }}.create(result)
		end
		{{- end }}
		return result
	end)
end
	{{- end }}
{{- end }}

return M
`

var schema struct {
	Paths map[string]map[string]struct {
		Summary     string
		OperationId string
		Responses   struct {
			Ok struct {
				Schema struct {
					Ref string `json:"$ref"`
				}
			} `json:"200"`
		}
		Parameters []struct {
			Name     	string
			Description	string
			In       	string
			Required 	bool
			Type     	string   // used with primitives
			Items    	struct { // used with type "array"
				Type string
			}
			Schema struct { // used with http body
				Type string
				Ref  string `json:"$ref"`
			}
			Format   string // used with type "boolean"
		}
		Security []map[string][]struct {
		}
	}
	Definitions map[string]struct {
		Properties map[string]struct {
			Type  string
			Ref   string   `json:"$ref"` // used with object
			Items struct { // used with type "array"
				Type string
				Ref  string `json:"$ref"`
			}
			AdditionalProperties struct {
				Type string // used with type "map"
			}
			Format      string // used with type "boolean"
			Description string
		}
		Enum        []string
		Description string
		// used only by enums
		Title string
	}
}

func convertRefToClassName(input string) (className string) {
	cleanRef := strings.TrimPrefix(input, "#/definitions/")
	className = strings.Title(cleanRef)
	return
}

func stripNewlines(input string) (output string) {
	output = strings.Replace(input, "\n", "\n--", -1)
	return
}

func pascalToSnake(input string) (output string) {
	output = ""
	prev_low := false
	for _, v := range input {
		is_cap := v >= 'A' && v <= 'Z'
		is_low := v >= 'a' && v <= 'z'
		if is_cap && prev_low {
			output = output + "_"
		}
		output += strings.ToLower(string(v))
		prev_low = is_low
	}
	return
}

// camelToPascal converts a string from camel case to Pascal case.
func camelToPascal(camelCase string) (pascalCase string) {
	if len(camelCase) <= 0 {
		return ""
	}
	pascalCase = strings.ToUpper(string(camelCase[0])) + camelCase[1:]
	return
}
// pascalToCamel converts a Pascal case string to a camel case string.
func pascalToCamel(input string) (camelCase string) {
	if input == "" {
		return ""
	}
	camelCase = strings.ToLower(string(input[0]))
	camelCase += string(input[1:])
	return camelCase
}

func removePrefix(input string) (output string) {
	output = strings.Replace(input, "nakama_", "", -1)
	return
}

func isEnum(ref string) bool {
	// swagger schema definition keys have inconsistent casing
	var camelOk bool
	var pascalOk bool
	var enums []string

	cleanedRef := convertRefToClassName(ref)
	asCamel := pascalToCamel(cleanedRef)
	if _, camelOk = schema.Definitions[asCamel]; camelOk {
		enums = schema.Definitions[asCamel].Enum
	}

	asPascal := camelToPascal(cleanedRef)
	if _, pascalOk = schema.Definitions[asPascal]; pascalOk {
		enums = schema.Definitions[asPascal].Enum
	}

	if !pascalOk && !camelOk {
		return false
	}

	return len(enums) > 0
}

// Parameter type to Lua type
func luaType(p_type string, p_ref string) (out string) {
	if isEnum(p_ref) {
		out = "string"
		return
	}
	switch p_type {
		case "integer": out = "number"
		case "string": out = "string"
		case "boolean": out = "boolean"
		case "array": out = "table"
		case "object": out = "table"
		default: out = "table"
	}
	return
}

// Default value for Lua types
func luaDef(p_type string, p_ref string) (out string) {
	switch(p_type) {
		case "integer": out = "0"
		case "string": out = "\"\""
		case "boolean": out = "false"
		case "array": out = "{}"
		case "object": out = "{ _ = '' }"
		default: out = "M.create_" + pascalToSnake(convertRefToClassName(p_ref)) + "()"
	}
	return
}

// Lua variable name from name, type and ref
func varName(p_name string, p_type string, p_ref string) (out string) {
	switch(p_type) {
		case "integer": out = p_name + "_int"
		case "string": out = p_name + "_str"
		case "boolean": out = p_name + "_bool"
		case "array": out = p_name + "_arr"
		case "object": out = p_name + "_obj"
		default: out = p_name + "_" + pascalToSnake(convertRefToClassName(p_ref))
	}
	return
}

func varComment(p_name string, p_type string, p_ref string, p_item_type string) (out string) {
	switch(p_type) {
		case "integer": out = "number"
		case "string": out = "string"
		case "boolean": out = "boolean"
		case "array": out = "table (" + luaType(p_item_type, p_ref) + ")"
		case "object": out = "table (object)"
		default: out = "table (" + pascalToSnake(convertRefToClassName(p_ref)) + ")"
	}
	return
}

func isAuthenticateMethod(input string) (output bool) {
	output = strings.HasPrefix(input, "Nakama_Authenticate")
	return
}

func main() {
	// Argument flags
	var output = flag.String("output", "", "The output for generated code.")
	flag.Parse()

	inputs := flag.Args()
	if len(inputs) < 1 {
		fmt.Printf("No input file found: %s\n\n", inputs)
		fmt.Println("openapi-gen [flags] inputs...")
		flag.PrintDefaults()
		return
	}

	input := inputs[0]
	content, err := ioutil.ReadFile(input)
	if err != nil {
		fmt.Printf("Unable to read file: %s\n", err)
		return
	}


	if err := json.Unmarshal(content, &schema); err != nil {
		fmt.Printf("Unable to decode input %s : %s\n", input, err)
		return
	}


	// expand the body argument to individual function arguments
	bodyFunctionArgs := func(ref string) (output string) {
		ref = strings.Replace(ref, "#/definitions/", "", -1)
		props := schema.Definitions[ref].Properties
		keys := make([]string, 0, len(props))
		for prop := range props {
			keys = append(keys, prop)
		}
		sort.Strings(keys)
		for _,key := range keys {
			output = output + ", " + key
		}
		return
	}

	// expand the body argument to individual function argument docs
	bodyFunctionArgsDocs := func(ref string) (output string) {
		ref = strings.Replace(ref, "#/definitions/", "", -1)
		output = "\n"
		props := schema.Definitions[ref].Properties
		keys := make([]string, 0, len(props))
		for prop := range props {
			keys = append(keys, prop)
		}
		sort.Strings(keys)
		for _,key := range keys {
			info := props[key]
			output = output + "-- @param " + key + " (" + info.Type + ") " + stripNewlines(info.Description) + "\n"
		}
		return
	}

	// expand the body argument to individual asserts for the call args
	bodyFunctionArgsAssert := func(ref string) (output string) {
		ref = strings.Replace(ref, "#/definitions/", "", -1)
		output = "\n"
		props := schema.Definitions[ref].Properties
		keys := make([]string, 0, len(props))
		for prop := range props {
			keys = append(keys, prop)
		}
		sort.Strings(keys)
		for _,key := range keys {
			info := props[key]
			luaType := luaType(info.Type, info.Ref)
			output = output + "\tassert(not " + key + " or type(" + key + ") == \"" + luaType + "\", \"Argument '" + key + "' must be 'nil' or of type '" + luaType + "'\")\n"
		}
		return
	}

	// expand the body argument to individual asserts for the message body table
	bodyFunctionArgsTable := func(ref string) (output string) {
		ref = strings.Replace(ref, "#/definitions/", "", -1)
		output = "\n"
		props := schema.Definitions[ref].Properties
		keys := make([]string, 0, len(props))
		for prop := range props {
			keys = append(keys, prop)
		}
		sort.Strings(keys)
		for _,key := range keys {
			output = output + "\t" + key + " = " + key + ",\n"
		}
		return
	}

	fmap := template.FuncMap{
		"cleanRef": convertRefToClassName,
		"stripNewlines": stripNewlines,
		"title": strings.Title,
		"uppercase": strings.ToUpper,
		"pascalToSnake": pascalToSnake,
		"luaType": luaType,
		"luaDef": luaDef,
		"varName": varName,
		"varComment": varComment,
		"bodyFunctionArgsDocs": bodyFunctionArgsDocs,
		"bodyFunctionArgs": bodyFunctionArgs,
		"bodyFunctionArgsAssert": bodyFunctionArgsAssert,
		"bodyFunctionArgsTable": bodyFunctionArgsTable,
		"isEnum": isEnum,
		"isAuthenticateMethod": isAuthenticateMethod,
		"removePrefix": removePrefix,
	}
	tmpl, err := template.New(input).Funcs(fmap).Parse(codeTemplate)
	if err != nil {
		fmt.Printf("Template parse error: %s\n", err)
		return
	}

	if len(*output) < 1 {
		tmpl.Execute(os.Stdout, schema)
		return
	}

	f, err := os.Create(*output)
	if err != nil {
		fmt.Printf("Unable to create file: %s\n", err)
		return
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	tmpl.Execute(writer, schema)
	writer.Flush()
}
