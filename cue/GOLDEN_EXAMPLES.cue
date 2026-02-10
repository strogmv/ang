// ============================================================================
// GOLDEN EXAMPLES - Canonical Flow DSL Patterns
// ============================================================================
// This file contains reference implementations for common operations.
// AI Agents: Use these as templates for generating correct Flow DSL.
// ============================================================================

package examples

import "github.com/strogmv/ang/cue/schema"

// ============================================================================
// EXAMPLE 1: Create Entity
// ============================================================================
// Pattern: Create new entity with ownership and timestamps
// Generated Go:
//   var newOrder domain.Order
//   newOrder.ID = uuid.NewString()
//   newOrder.UserID = req.UserID
//   newOrder.Status = "draft"
//   newOrder.CreatedAt = time.Now().UTC().Format(time.RFC3339)
//   newOrder.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
//   if err := s.OrderRepo.Save(ctx, &newOrder); err != nil { return resp, err }
//   resp.ID = newOrder.ID

CreateOrder: schema.#Operation & {
	service: "orders"
	input: {
		userID: string
		title:  string
		amount: int
	}
	output: {
		id: string
	}
	flow: [
		{action: "mapping.Map", output: "newOrder", entity: "Order"},
		{action: "mapping.Assign", to: "newOrder.UserID", value: "req.UserID"},
		{action: "mapping.Assign", to: "newOrder.Title", value: "req.Title"},
		{action: "mapping.Assign", to: "newOrder.Amount", value: "req.Amount"},
		{action: "mapping.Assign", to: "newOrder.Status", value: "\"draft\""},
		{action: "repo.Save", source: "Order", input: "newOrder"},
		{action: "mapping.Assign", to: "resp.ID", value: "newOrder.ID"},
	]
}

// ============================================================================
// EXAMPLE 2: Update Entity with Ownership Check
// ============================================================================
// Pattern: Find, verify ownership, update in transaction
// Generated Go:
//   order, err := s.OrderRepo.FindByID(ctx, req.OrderID)
//   if err != nil { return resp, err }
//   if order == nil { return resp, errors.New(404, "Not Found", "Order not found") }
//   if order.UserID != req.UserID { return resp, errors.New(403, "Forbidden", "Access denied") }
//   err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
//       order.Title = req.Title
//       order.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
//       return s.OrderRepo.Save(txCtx, order)
//   })

UpdateOrder: schema.#Operation & {
	service: "orders"
	input: {
		orderID: string
		userID:  string
		title:   string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order", error: "Order not found"},
		{action: "logic.Check", condition: "order.UserID == req.UserID", throw: "Access denied"},
		{action: "tx.Block", do: [
			{action: "mapping.Assign", to: "order.Title", value: "req.Title"},
			{action: "mapping.Assign", to: "order.UpdatedAt", value: "time.Now().UTC().Format(time.RFC3339)"},
			{action: "repo.Save", source: "Order", input: "order"},
		]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 3: State Machine Transition
// ============================================================================
// Pattern: Validate status, transition FSM, publish event
// Generated Go:
//   order, err := s.OrderRepo.FindByID(ctx, req.OrderID)
//   if order.Status != "draft" { return resp, errors.New(400, "Bad Request", "Order must be in draft") }
//   if err := order.TransitionTo("confirmed"); err != nil { return resp, err }
//   s.OrderRepo.Save(ctx, order)
//   s.publisher.Publish("OrderConfirmed", domain.OrderConfirmed{OrderID: order.ID})

ConfirmOrder: schema.#Operation & {
	service:   "orders"
	publishes: ["OrderConfirmed"]
	input: {
		orderID: string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order", error: "Order not found"},
		{action: "logic.Check", condition: "order.Status == \"draft\"", throw: "Order must be in draft status"},
		{action: "fsm.Transition", entity: "order", to: "confirmed"},
		{action: "repo.Save", source: "Order", input: "order"},
		{action: "event.Publish", name: "OrderConfirmed", payload: "domain.OrderConfirmed{OrderID: order.ID}"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 4: List with Pagination
// ============================================================================
// Pattern: List entities for user with offset/limit
// Generated Go:
//   orders, err := s.OrderRepo.ListByUser(ctx, req.UserID, req.Offset, req.Limit)
//   resp.Data = orders
//   resp.Total = len(orders)

ListOrders: schema.#Operation & {
	service: "orders"
	pagination: {
		type:          "offset"
		default_limit: 20
		max_limit:     100
	}
	input: {
		userID: string
	}
	output: {
		data: [...{}]
		total: int
	}
	flow: [
		{action: "repo.List", source: "Order", method: "ListByUser", input: "req.UserID", output: "orders"},
		{action: "mapping.Assign", to: "resp.Data", value: "orders"},
		{action: "mapping.Assign", to: "resp.Total", value: "len(orders)"},
	]
}

// ============================================================================
// EXAMPLE 5: Conditional Logic
// ============================================================================
// Pattern: Different behavior based on condition
// Generated Go:
//   if user.Role == "admin" {
//       orders, _ = s.OrderRepo.ListAll(ctx, 0, 100)
//   } else {
//       orders, _ = s.OrderRepo.ListByUser(ctx, req.UserID, 0, 100)
//   }

ListOrdersConditional: schema.#Operation & {
	service: "orders"
	input: {
		userID: string
	}
	output: {
		data: [...{}]
	}
	flow: [
		{action: "repo.Find", source: "User", input: "req.UserID", output: "user"},
		{action: "flow.If", condition: "user.Role == \"admin\"", then: [
			{action: "repo.List", source: "Order", method: "ListAll", output: "orders"},
		], else: [
			{action: "repo.List", source: "Order", method: "ListByUser", input: "req.UserID", output: "orders"},
		]},
		{action: "mapping.Assign", to: "resp.Data", value: "orders"},
	]
}

// ============================================================================
// EXAMPLE 6: Loop / Batch Processing
// ============================================================================
// Pattern: Process multiple items in a loop
// Generated Go:
//   for _, itemReq := range req.Items {
//       var newItem domain.Item
//       newItem.ID = uuid.NewString()
//       newItem.OrderID = req.OrderID
//       newItem.Name = itemReq.Name
//       s.ItemRepo.Save(ctx, &newItem)
//   }

AddOrderItems: schema.#Operation & {
	service: "orders"
	input: {
		orderID: string
		items: [...{name: string, qty: int}]
	}
	output: {
		count: int
	}
	flow: [
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order", error: "Order not found"},
		{action: "flow.For", each: "req.Items", as: "itemReq", do: [
			{action: "mapping.Map", output: "newItem", entity: "Item"},
			{action: "mapping.Assign", to: "newItem.OrderID", value: "order.ID"},
			{action: "mapping.Assign", to: "newItem.Name", value: "itemReq.Name"},
			{action: "mapping.Assign", to: "newItem.Qty", value: "itemReq.Qty"},
			{action: "repo.Save", source: "Item", input: "newItem"},
		]},
		{action: "mapping.Assign", to: "resp.Count", value: "len(req.Items)"},
	]
}

// ============================================================================
// EXAMPLE 7: Delete with Cascade Check
// ============================================================================
// Pattern: Check dependencies before delete
// Generated Go:
//   order, _ := s.OrderRepo.FindByID(ctx, req.OrderID)
//   if order.Status != "draft" { return resp, errors.New(400, "Cannot delete") }
//   items, _ := s.ItemRepo.ListByOrder(ctx, order.ID)
//   if len(items) > 0 { return resp, errors.New(400, "Order has items") }
//   s.OrderRepo.Delete(ctx, order.ID)

DeleteOrder: schema.#Operation & {
	service: "orders"
	input: {
		orderID: string
		userID:  string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order", error: "Order not found"},
		{action: "logic.Check", condition: "order.UserID == req.UserID", throw: "Access denied"},
		{action: "logic.Check", condition: "order.Status == \"draft\"", throw: "Only draft orders can be deleted"},
		{action: "repo.List", source: "Item", method: "ListByOrder", input: "order.ID", output: "items"},
		{action: "logic.Check", condition: "len(items) == 0", throw: "Order has items, delete them first"},
		{action: "repo.Delete", source: "Order", input: "order.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 8: Webhook Handler (Stripe)
// ============================================================================
// Pattern: Public webhook endpoint with signature check and event dispatch
// Generated Go:
//   payload := req.RawBody
//   if !stripe.VerifySignature(payload, req.Signature, cfg.StripeSecret) { return 401 }
//   evt := stripe.ParseEvent(payload)
//   if evt.Type == "payment_intent.succeeded" { s.publisher.Publish("PaymentSucceeded", ...) }

HandleStripeWebhook: schema.#Operation & {
	service: "payments"
	input: {
		payload:   string
		signature: string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "logic.Check", condition: "verifyStripeSignature(req.Payload, req.Signature)", throw: "Invalid Stripe signature"},
		{action: "mapping.Assign", to: "eventType", value: "extractStripeEventType(req.Payload)"},
		{action: "flow.If", condition: "eventType == \"payment_intent.succeeded\"", then: [
			{action: "event.Publish", name: "PaymentSucceeded", payload: "domain.PaymentSucceeded{Raw: req.Payload}"},
		]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 9: File Upload (@image + S3)
// ============================================================================
// Pattern: Validate image metadata and store file via storage port
// Generated Go:
//   if !isImage(req.ContentType) { return 400 }
//   key, err := s.storage.PutObject(ctx, req.Bytes, "products/"+req.ProductID)
//   product.ImageURL = key

UploadProductImage: schema.#Operation & {
	service: "catalog"
	input: {
		productID:   string
		fileName:    string
		contentType: string
		bytes:       string
	}
	output: {
		url: string
	}
	flow: [
		{action: "logic.Check", condition: "isSupportedImage(req.ContentType)", throw: "Unsupported image type"},
		{action: "repo.Find", source: "Product", input: "req.ProductID", output: "product", error: "Product not found"},
		{action: "storage.Upload", source: "s3", input: "req.Bytes", output: "fileURL"},
		{action: "mapping.Assign", to: "product.ImageURL", value: "fileURL"},
		{action: "repo.Save", source: "Product", input: "product"},
		{action: "mapping.Assign", to: "resp.URL", value: "fileURL"},
	]
}

// ============================================================================
// EXAMPLE 10: Caching with TTL
// ============================================================================
// Pattern: Read-through cache for hot endpoint
// Generated Go:
//   if cached, ok := cache.Get(key); ok { return cached }
//   data := repo.Get(...)
//   cache.Set(key, data, 5*time.Minute)

GetDashboard: schema.#Operation & {
	service: "analytics"
	input: {
		userID: string
	}
	output: {
		data: {}
	}
	flow: [
		{action: "cache.Get", key: "\"dashboard:\"+req.UserID", output: "cached"},
		{action: "flow.If", condition: "cached != nil", then: [
			{action: "mapping.Assign", to: "resp.Data", value: "cached"},
		], else: [
			{action: "repo.Find", source: "Dashboard", input: "req.UserID", output: "dash"},
			{action: "cache.Set", key: "\"dashboard:\"+req.UserID", value: "dash", ttl: "\"5m\""},
			{action: "mapping.Assign", to: "resp.Data", value: "dash"},
		]},
	]
}

// ============================================================================
// EXAMPLE 11: Rate Limiting
// ============================================================================
// Pattern: Endpoint-level limiter with RPS/burst policy
// Generated Go:
//   limiter := limiter.ForKey(req.UserID)
//   if !limiter.Allow() { return 429 }

CreateCheckoutSession: schema.#Operation & {
	service: "payments"
	input: {
		userID: string
		orderID: string
	}
	output: {
		sessionURL: string
	}
	flow: [
		{action: "rateLimit.Check", key: "req.UserID", rps: 2, burst: 4},
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order", error: "Order not found"},
		{action: "mapping.Assign", to: "resp.SessionURL", value: "createCheckoutURL(order)"},
	]
}

// ============================================================================
// EXAMPLE 12: Multi-Entity Transaction
// ============================================================================
// Pattern: One tx.Block updates inventory, order, and payment ledger
// Generated Go:
//   tx.WithTx(ctx, func(txCtx) {
//     inventory.Reserved += req.Qty
//     order.Status = "paid"
//     ledger.Insert(...)
//   })

ConfirmPaymentTx: schema.#Operation & {
	service: "orders"
	input: {
		orderID: string
		qty: int
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "tx.Block", do: [
			{action: "repo.Find", source: "Inventory", input: "req.OrderID", output: "inv"},
			{action: "mapping.Assign", to: "inv.Reserved", value: "inv.Reserved + req.Qty"},
			{action: "repo.Save", source: "Inventory", input: "inv"},
			{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order"},
			{action: "mapping.Assign", to: "order.Status", value: "\"paid\""},
			{action: "repo.Save", source: "Order", input: "order"},
			{action: "repo.Save", source: "PaymentLedger", input: "domain.PaymentLedger{OrderID: order.ID, Amount: order.Amount}"},
		]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 13: Event -> Handler -> Side Effect Chain
// ============================================================================
// Pattern: Consume event and trigger mail side effect
// Generated Go:
//   on OrderPaid -> load seller -> send email -> mark notification sent

NotifySellerOnOrderPaid: schema.#Operation & {
	service: "notifications"
	input: {
		orderID: string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order"},
		{action: "repo.Find", source: "User", input: "order.SellerID", output: "seller"},
		{action: "mailer.Send", input: "seller.Email", template: "\"order_paid\""},
		{action: "repo.Save", source: "NotificationLog", input: "domain.NotificationLog{OrderID: order.ID, Kind: \"order_paid\"}"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 14: Search / Filter
// ============================================================================
// Pattern: Complex finder with query + range + status
// Generated Go:
//   products, err := repo.Search(ctx, req.Query, req.MinPrice, req.MaxPrice, req.Status)

SearchProducts: schema.#Operation & {
	service: "catalog"
	input: {
		query: string
		minPrice: int
		maxPrice: int
		status: string
	}
	output: {
		data: [...{}]
	}
	flow: [
		{action: "repo.List", source: "Product", method: "SearchByQueryPriceStatus", input: "req", output: "items"},
		{action: "mapping.Assign", to: "resp.Data", value: "items"},
	]
}

// ============================================================================
// EXAMPLE 15: Aggregation / Report
// ============================================================================
// Pattern: COUNT + SUM style report endpoint
// Generated Go:
//   report := repo.SalesReport(ctx, req.From, req.To)
//   resp.TotalOrders = report.Count
//   resp.TotalRevenue = report.Sum

GetSalesReport: schema.#Operation & {
	service: "analytics"
	input: {
		from: string
		to: string
	}
	output: {
		totalOrders: int
		totalRevenue: int
	}
	flow: [
		{action: "repo.Find", source: "SalesReport", method: "AggregateByRange", input: "req", output: "r"},
		{action: "mapping.Assign", to: "resp.TotalOrders", value: "r.TotalOrders"},
		{action: "mapping.Assign", to: "resp.TotalRevenue", value: "r.TotalRevenue"},
	]
}

// ============================================================================
// EXAMPLE 16: Scheduled Job
// ============================================================================
// Pattern: cron tick publishes maintenance event
// Generated Go:
//   scheduler.Every("0 * * * *", func(){ publisher.Publish("CleanupExpiredCarts", ...) })

RunHourlyCartCleanup: schema.#Operation & {
	service: "scheduler"
	input: {}
	output: {
		ok: bool
	}
	flow: [
		{action: "event.Publish", name: "CleanupExpiredCarts", payload: "domain.CleanupExpiredCarts{}"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 17: Multi-Step Workflow (Saga)
// ============================================================================
// Pattern: Reserve inventory -> charge payment -> confirm order / compensating action
// Generated Go:
//   reserve()
//   if charge fails -> release reserve
//   else confirm order

PlaceOrderSaga: schema.#Operation & {
	service: "orders"
	input: {
		orderID: string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order"},
		{action: "flow.If", condition: "reserveInventory(order)", then: [
			{action: "flow.If", condition: "chargePayment(order)", then: [
				{action: "mapping.Assign", to: "order.Status", value: "\"paid\""},
				{action: "repo.Save", source: "Order", input: "order"},
			], else: [
				{action: "logic.Call", name: "releaseInventory", args: ["order"]},
			]},
		]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 18: Nested Resource (/orders/{id}/items)
// ============================================================================
// Pattern: Child resource mutation under parent scope
// Generated Go:
//   order := repo.Find(orderID)
//   item := map(req)
//   item.OrderID = order.ID
//   repo.Save(item)

AddOrderItemNested: schema.#Operation & {
	service: "orders"
	input: {
		orderID: string
		name: string
		qty: int
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order", error: "Order not found"},
		{action: "mapping.Map", output: "item", entity: "Item"},
		{action: "mapping.Assign", to: "item.OrderID", value: "order.ID"},
		{action: "mapping.Assign", to: "item.Name", value: "req.Name"},
		{action: "mapping.Assign", to: "item.Qty", value: "req.Qty"},
		{action: "repo.Save", source: "Item", input: "item"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 19: Soft Delete
// ============================================================================
// Pattern: Mark entity as deleted instead of hard delete
// Generated Go:
//   product.DeletedAt = now
//   product.DeletedBy = req.UserID
//   repo.Save(product)

SoftDeleteProduct: schema.#Operation & {
	service: "catalog"
	input: {
		productID: string
		userID: string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "repo.Find", source: "Product", input: "req.ProductID", output: "product"},
		{action: "mapping.Assign", to: "product.DeletedAt", value: "time.Now().UTC().Format(time.RFC3339)"},
		{action: "mapping.Assign", to: "product.DeletedBy", value: "req.UserID"},
		{action: "repo.Save", source: "Product", input: "product"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ============================================================================
// EXAMPLE 20: Audit Trail
// ============================================================================
// Pattern: Persist audit record for state-changing action
// Generated Go:
//   before := snapshot(order)
//   mutate(order)
//   repo.Save(order)
//   auditRepo.Save({...before, after, actor})

UpdateOrderWithAudit: schema.#Operation & {
	service: "orders"
	input: {
		orderID: string
		userID: string
		status: string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "repo.Find", source: "Order", input: "req.OrderID", output: "order"},
		{action: "mapping.Assign", to: "before", value: "snapshot(order)"},
		{action: "mapping.Assign", to: "order.Status", value: "req.Status"},
		{action: "repo.Save", source: "Order", input: "order"},
		{action: "repo.Save", source: "AuditLog", input: "domain.AuditLog{Entity: \"Order\", EntityID: order.ID, ActorID: req.UserID, Before: before, After: snapshot(order)}"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}
