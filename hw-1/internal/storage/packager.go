package storage

import "fmt"

type Packager interface {
	ValidateWeight(weight float32) error
	AdjustPrice(price int) int
	GetType() string
}

type BasePackager struct {
	Type Container
}

func (b BasePackager) GetType() string {
	return string(b.Type)
}

type BagPackager struct {
	BasePackager
}

func NewBagPackager() *BagPackager {
	return &BagPackager{BasePackager{Type: Bag}}
}

func (b BagPackager) ValidateWeight(weight float32) error {
	if weight >= 10 {
		return fmt.Errorf("weight exceeds maximum for Bag packaging: %.2f kg >= 10 kg", weight)
	}
	return nil
}

func (b BagPackager) AdjustPrice(price int) int {
	return price + 5
}

type BoxPackager struct {
	BasePackager
}

func NewBoxPackager() *BoxPackager {
	return &BoxPackager{BasePackager{Type: Box}}
}

func (b BoxPackager) ValidateWeight(weight float32) error {
	if weight >= 30 {
		return fmt.Errorf("weight exceeds maximum for Box packaging: %.2f kg >= 30 kg", weight)
	}
	return nil
}

func (b BoxPackager) AdjustPrice(price int) int {
	return price + 20
}

type MembranePackager struct {
	BasePackager
}

func NewMembranePackager() *MembranePackager {
	return &MembranePackager{BasePackager{Type: Membrane}}
}

func (m MembranePackager) ValidateWeight(weight float32) error {
	return nil
}

func (m MembranePackager) AdjustPrice(price int) int {
	return price + 1
}

type MembraneDecorator struct {
	Packager Packager
}

func NewMembraneDecorator(packager Packager) *MembraneDecorator {
	return &MembraneDecorator{Packager: packager}
}

func (m MembraneDecorator) ValidateWeight(weight float32) error {
	return m.Packager.ValidateWeight(weight)
}

func (m MembraneDecorator) AdjustPrice(price int) int {
	return m.Packager.AdjustPrice(price) + 1
}

func (m MembraneDecorator) GetType() string {
	return m.Packager.GetType() + " with Membrane"
}

func GetPackager(wrapperType string, withMembrane bool) (Packager, error) {
	var packager Packager

	switch Container(wrapperType) {
	case Bag:
		packager = NewBagPackager()
	case Box:
		packager = NewBoxPackager()
	case Membrane:
		packager = NewMembranePackager()
	default:
		return nil, fmt.Errorf("unknown wrapper type: %s", wrapperType)
	}

	if withMembrane && wrapperType != string(Membrane) {
		packager = NewMembraneDecorator(packager)
	}

	return packager, nil
}
