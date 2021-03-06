package compute

import "fmt"

type Network struct {
	client networksClient
	name   string
}

func NewNetwork(client networksClient, name string) Network {
	return Network{
		client: client,
		name:   name,
	}
}

func (n Network) Delete() error {
	err := n.client.DeleteNetwork(n.name)

	if err != nil {
		return fmt.Errorf("ERROR deleting network %s: %s", n.name, err)
	}

	return nil
}
