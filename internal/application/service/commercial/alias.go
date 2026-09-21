// Pass A compatibility alias for internal/modules/commercial/service/commercial — zero logic. Deleted by Pass B task B-commercial.

package commercial

import (
	commercial "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
)

type BenefitsService = commercial.BenefitsService

type BillingAccountService = commercial.BillingAccountService

type FulfillmentService = commercial.FulfillmentService

type OrderService = commercial.OrderService

type PlanVersionService = commercial.PlanVersionService

var NewBenefitsService = commercial.NewBenefitsService

var NewBillingAccountService = commercial.NewBillingAccountService

var NewExecutionGateService = commercial.NewExecutionGateService

var NewFulfillmentService = commercial.NewFulfillmentService

var NewOrderService = commercial.NewOrderService

var NewPlanVersionService = commercial.NewPlanVersionService

var NewQuotaGuard = commercial.NewQuotaGuard

var NewRecoveryService = commercial.NewRecoveryService
