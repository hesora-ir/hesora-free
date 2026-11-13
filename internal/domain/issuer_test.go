package domain

import "testing"

func TestIssuerProfileValidationByType(t *testing.T) {
	tests := []struct {
		name      string
		profile   IssuerProfile
		wantError bool
	}{
		{name: "individual requires a name", profile: IssuerProfile{Type: IssuerTypeIndividual}, wantError: true},
		{name: "individual accepts full name", profile: IssuerProfile{Type: IssuerTypeIndividual, Name: "نام کامل", FirstName: "نام", LastName: "خانوادگی"}},
		{name: "company requires legal name", profile: IssuerProfile{Type: IssuerTypeCompany, Name: "شرکت"}, wantError: true},
		{name: "company accepts legal name", profile: IssuerProfile{Type: IssuerTypeCompany, Name: "شرکت نمونه", LegalName: "شرکت نمونه"}},
		{name: "empty bank row rejected", profile: IssuerProfile{Type: IssuerTypeIndividual, Name: "نام کامل", FirstName: "نام", BankDetails: []IssuerBankDetails{{}}}, wantError: true},
		{name: "partial bank row accepted", profile: IssuerProfile{Type: IssuerTypeIndividual, Name: "نام کامل", FirstName: "نام", BankDetails: []IssuerBankDetails{{AccountNumber: "account-test"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantError {
				t.Fatalf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
