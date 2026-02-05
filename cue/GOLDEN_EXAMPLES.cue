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
