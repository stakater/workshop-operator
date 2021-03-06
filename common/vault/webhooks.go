package vault

import (
	admissionregistration "k8s.io/api/admissionregistration/v1"
)

// NewAgentInjectorWebHook create webhook
func NewAgentInjectorWebHook(namespace string) []admissionregistration.MutatingWebhook {
	path := "/mutate"

	return []admissionregistration.MutatingWebhook{
		{
			Name: "vault.hashicorp.com",
			ClientConfig: admissionregistration.WebhookClientConfig{
				CABundle: []byte{},
				Service: &admissionregistration.ServiceReference{
					Name:      "vault-agent-injector",
					Namespace: namespace,
					Path:      &path,
				},
			},
			SideEffects:             sideEffect(),
			FailurePolicy:           failurePolicy(),
			AdmissionReviewVersions: []string{"v1"},
			Rules: []admissionregistration.RuleWithOperations{
				{
					Operations: []admissionregistration.OperationType{
						"CREATE",
						"UPDATE",
					},
					Rule: admissionregistration.Rule{
						APIGroups: []string{
							"",
						},
						APIVersions: []string{
							"v1",
						},
						Resources: []string{
							"pods",
						},
					},
				},
			},
		},
	}
}

func sideEffect() *admissionregistration.SideEffectClass {
	sideEffectClass := admissionregistration.SideEffectClassNoneOnDryRun
	return &sideEffectClass
}

func failurePolicy() *admissionregistration.FailurePolicyType {
	failurePolicyType := admissionregistration.Ignore
	return &failurePolicyType
}
