package fsutils

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
)

// Render create the cs required inventory file based on the cluster config
func Render(tpl string, config *map[string]string) string {
	t, err := template.New("invTpl").Parse(tpl)
	if err != nil {
		panic(err)
	}

	return process(t, config)
}

// process applies the data structure 'vars' onto an already
// parsed template 't', and returns the resulting string.
func process(t *template.Template, vars interface{}) string {
	var tmplBytes bytes.Buffer

	err := t.Execute(&tmplBytes, vars)
	if err != nil {
		panic(err)
	}
	return tmplBytes.String()
}

func protect(g func()) {
	defer func() {
		if x := recover(); x != nil {
			log.Printf("run time panic: %v", x)
		}
	}()
	g()
}

// NormalizeName make the name space path into _ name
func NormalizeName(path string) string {
	return strings.ReplaceAll(path, " ", "_")
}

// MapStringKeys get the keys of an map as string slice
// will panic if not an map
func MapStringKeys(v interface{}) []string {
	keys := []string{}
	value := reflect.ValueOf(v)
	if value.Kind() == reflect.Map {
		for _, v := range value.MapKeys() {
			if v.Kind() == reflect.String {
				keys = append(keys, v.String())
			} else {
				panic("map key type is not string")
			}
		}
		return keys
	}
	panic("it is not a map!!")
}

// ShellToUse set the entry of the process command
const ShellToUse = "bash"

// ExecCmdInShell execute the various commands, and gather their stdout, stderr into string
func ExecCmdInShell(command string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(ShellToUse, "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz+/"

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	return b
}

// RandomString return a random string of length n
func RandomString(n int) string {
	var b = randomBytes(n)
	for i, _b := range b {
		b[i] = letters[_b%64]
	}

	return string(b)
}

// DateFormat ...
func DateFormat(layout string, d float64) string {
	intTime := int64(d)
	t := time.Unix(intTime, 0)
	if layout == "" {
		layout = "2006-01-02 15:04:05"
	}
	return t.Format(layout)
}

// ValidateSort validate the given sort against the given struct
// if the given struct contains the given sort keyword, it's valid else invalid
func ValidateSort(ptr interface{}, sortable Sortable) error {
	ptrt := reflect.TypeOf(ptr)
	if ptrt.Kind() != reflect.Ptr {
		return fmt.Errorf("The input interface is not ptr: %+v", ptr)
	}
	elet := ptrt.Elem()
	if elet.Kind() != reflect.Struct {
		return fmt.Errorf("The input interface is not struct: %+v", ptr)
	}
	elev := reflect.ValueOf(ptr).Elem()

	for i := 0; i < elev.NumField(); i++ {
		fieldt := elet.Field(i)
		tagName := fieldt.Tag.Get("json")
		if tagName == sortable.Sort {
			return nil
		}
	}

	return ErrNoSuchSort
}

// TagSortFunc return a sorter based on the given filed and asc
func TagSortFunc(tag string, asc bool) func(p1, p2 interface{}) bool {
	return func(p1, p2 interface{}) bool {
		elems := reflect.TypeOf(p1).Elem()

		var field string
		for i := 0; i < elems.NumField(); i++ {
			if elems.Field(i).Tag.Get("json") == tag {
				field = elems.Field(i).Name
				break
			}
		}
		v1 := reflect.Indirect(reflect.ValueOf(p1)).FieldByName(field)
		v2 := reflect.Indirect(reflect.ValueOf(p2)).FieldByName(field)

		ret := false

		switch v1.Kind() {
		case reflect.Int64:
			ret = v1.Int() < v2.Int()
		case reflect.Float64:
			ret = v1.Float() < v2.Float()
		case reflect.String:
			ret = v1.String() < v2.String()
		}

		if asc {
			return ret
		}
		return !ret
	}
}

func ToKiB(bytes int64) string {
	return fmt.Sprintf("%6.2fKB", float64(bytes)/float64(KiB))
}
func ToMiB(bytes int64) string {
	return fmt.Sprintf("%6.2fMB", float64(bytes)/float64(MiB))
}
func ToGiB(bytes int64) string {
	return fmt.Sprintf("%6.2fGB", float64(bytes)/float64(GiB))
}

func ToHumanBytes(bytes int64) string {
	if float64(bytes)/float64(MiB) > 1000 {
		return ToGiB(bytes)
	}
	if float64(bytes)/float64(KiB) > 1000 {
		return ToMiB(bytes)
	}
	if bytes > 1000 {
		return ToKiB(bytes)
	}
	return fmt.Sprintf("%6.2d B", bytes)
}

type (
	// PageSortable a combination of Pagable and Sortable
	PageSortable struct {
		Pagable
		Sortable
	}

	// Pagable represents a page request
	Pagable struct {
		Page int `json:"page"`
		Size int `json:"size"`
	}

	// Sortable represents a sort request
	Sortable struct {
		Sort string `json:"sort"`
		Asc  bool   `json:"asc"`
	}

	// BaseResponse ...
	BaseResponse struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}
)

var (
	// ErrNoSuchSort represents a invalid sort keyword is requested
	ErrNoSuchSort = errors.New("no such sort keyword")

	KiB = 1 << 10
	MiB = KiB << 10
	GiB = MiB << 10
)
