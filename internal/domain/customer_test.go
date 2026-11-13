package domain

import "testing"

func TestCustomerValidationByType(t *testing.T) {
	tests := []struct {
		name     string
		customer Customer
		wantErr  bool
	}{
		{name: "individual requires both names", customer: Customer{Type: CustomerTypeIndividual, FirstName: "نام"}, wantErr: true},
		{name: "individual accepts full name", customer: Customer{Type: CustomerTypeIndividual, FirstName: "نام", LastName: "خانوادگی"}},
		{name: "legacy display name remains valid", customer: Customer{Name: "مشتری قدیمی"}},
		{name: "company requires name", customer: Customer{Type: CustomerTypeCompany, TaxID: "123"}, wantErr: true},
		{name: "company accepts legal name", customer: Customer{Type: CustomerTypeCompany, LegalName: "شرکت نمونه"}},
		{name: "unknown type rejected", customer: Customer{Type: "other", Name: "مشتری"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.customer.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
