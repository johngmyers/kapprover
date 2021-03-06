package inspectors

import (
	"strings"
	"sync"

	"bytes"
	"errors"
	"fmt"
	certificates "k8s.io/api/certificates/v1beta1"
	"k8s.io/client-go/kubernetes"
)

var (
	inspectors = make(map[string]Inspector)
	inspectorM sync.RWMutex
)

// Inspector represents anything capable of performing a policy check on a CSR.
// It returns an empty string to take no action, a human readable message with details
// to take adverse action, or an error to temporarily fail.
type Inspector interface {
	Configure(string) (inspector Inspector, err error)
	Inspect(kubernetes.Interface, *certificates.CertificateSigningRequest) (message string, err error)
}

type NamedInspector struct {
	Name      string
	Config    string
	Inspector Inspector
}

// A slice of named Inspectors forming a policy.
type Inspectors []NamedInspector

func (inspectors *Inspectors) String() string {
	var b bytes.Buffer
	for idx, namedInspector := range *inspectors {
		if idx > 0 {
			b.WriteString(",")
		}
		b.WriteString(namedInspector.Name)
		if namedInspector.Config != "" {
			b.WriteString("=")
			b.WriteString(namedInspector.Config)
		}
	}
	return b.String()
}

func (inspectors *Inspectors) Set(value string) error {
	split := strings.SplitN(value, "=", 2)
	name := split[0]
	inspector, exists := Get(name)
	if !exists {
		return errors.New(fmt.Sprintf(
			"Could not find inspector %q, registered approvers: %s",
			name,
			strings.Join(List(), ","),
		))
	}
	var config string
	if len(split) > 1 {
		config = split[1]
	}
	if config != "" {
		var err error
		inspector, err = inspector.Configure(split[1])
		if err != nil {
			return err
		}
	}
	*inspectors = append(*inspectors, NamedInspector{Name: name, Config: config, Inspector: inspector})

	return nil
}

// Register makes an Inspector available by the provided name.
//
// If called twice with the same name, the name is blank, or if the provided
// Extractor is nil, this function panics.
func Register(name string, a Inspector) {
	inspectorM.Lock()
	defer inspectorM.Unlock()

	if name == "" {
		panic("inspectors: could not register an Inspector with an empty name")
	}

	if a == nil {
		panic("inspectors: could not register a nil Inspector")
	}

	// Enforce lowercase names, so that they can be reliably be found in a map.
	name = strings.ToLower(name)

	if _, dup := inspectors[name]; dup {
		panic("inspectors: RegisterApprover called twice for " + name)
	}

	inspectors[name] = a
}

// List returns the list of the registered inspectors' names.
func List() []string {
	inspectorM.RLock()
	defer inspectorM.RUnlock()

	ret := make([]string, 0, len(inspectors))
	for k := range inspectors {
		ret = append(ret, k)
	}

	return ret
}

// Unregister removes an Inspector with a particular name from the list.
func Unregister(name string) {
	inspectorM.Lock()
	defer inspectorM.Unlock()
	delete(inspectors, name)
}

// Get returns the registered Inspector with a provided name.
func Get(name string) (a Inspector, exists bool) {
	inspectorM.Lock()
	defer inspectorM.Unlock()

	a, exists = inspectors[name]
	return
}
