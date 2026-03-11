package schema

// Pack policies are declarative domain-shaping rules intended for upper-layer
// planners such as sandbox. ANG should validate and consume explicit canonical
// output from these rules, not re-implement domain inference in Go.

#HTTPMethod: "GET" | "POST" | "PUT" | "PATCH" | "DELETE"

#RepositoryBinding: {
	load_method?: string
	list_method?: string
	actor_field?: string
	input_field?: string
}

#RouteBinding: {
	method?: #HTTPMethod
	path:    string
}

#PackEntityMatch: {
	name_in?:      [...string]
	name_prefix?:  [...string]
	has_fields?:   [...string]
	owner_in?:     [...string]
	context_in?:   [...string]
	capability_in?: [...#CapabilityKind]
}

#PackOperationMatch: {
	name_in?:        [...string]
	name_prefix?:    [...string]
	method_in?:      [...#HTTPMethod]
	entity_in?:      [...string]
	has_input?:      [...string]
	has_output?:     [...string]
	has_entity?:     [...string]
	capability_in?:  [...#CapabilityKind]
	side_effect_in?: [...string]
}

#PackEntityRule: {
	match?: #PackEntityMatch
	capabilities?: [...#CapabilityKind]
	fields?: [...{
		name: string
		type: string
	}]
}

#PackOperationRule: {
	match: #PackOperationMatch

	primary_operation_kind?: #OperationKind
	capabilities?:           [...#CapabilityKind]
	side_effects?:           [...#SideEffect]
	manual_required?:        bool

	route?: #RouteBinding
	repository?: #RepositoryBinding
}

#PackPolicy: {
	name: string
	description?: string
	priority?: int | *100

	entity_rules?:    [...#PackEntityRule]
	operation_rules?: [...#PackOperationRule]
}

#PackPolicies: [string]: #PackPolicy

#PackRegistry: {
	// Declared pack rules that an upper-layer planner may apply.
	policies: #PackPolicies

	// Optional activation order/profile for deterministic planning.
	active?: [...string]

	// Conflict resolution strategy for multiple matching rules.
	conflict_strategy?: "highest_priority" | "first_match" | *"highest_priority"
}

// CommerceExample demonstrates how sandbox should declare domain knowledge in
// CUE instead of pushing it into ANG emitter heuristics.
#CommercePackExample: #PackPolicy & {
	name:        "commerce"
	description: "Canonical shaping rules for storefront, cart, checkout, order, and review flows."
	priority:    200

	entity_rules: [
		{
			match: {
				name_in: ["Product", "Category", "Customer", "Address", "Cart", "CartItem", "Order", "OrderItem", "Review"]
			}
			capabilities: ["commerce"]
		},
	]

	operation_rules: [
		{
			match: {
				name_prefix: ["CreateProduct", "CreateCategory"]
				entity_in:   ["Product", "Category"]
			}
			primary_operation_kind: "create"
			capabilities:           ["commerce", "catalog"]
			route: {method: "POST", path: "/products"}
		},
		{
			match: {
				name_prefix: ["ListProducts", "GetProduct", "ListCategories", "GetCategory"]
				entity_in:   ["Product", "Category"]
			}
			primary_operation_kind: "get"
			capabilities:           ["commerce", "catalog"]
			route: {method: "GET", path: "/products"}
		},
		{
			match: {
				name_prefix: ["AddToCart", "UpdateCartItem", "RemoveFromCart"]
				entity_in:   ["Cart", "CartItem"]
			}
			primary_operation_kind: "update"
			capabilities:           ["commerce"]
			route: {path: "/cart"}
			repository: {load_method: "FindByCustomerID", actor_field: "customerID"}
		},
		{
			match: {
				name_prefix: ["GetCart", "ListCartItems"]
				entity_in:   ["Cart", "CartItem"]
			}
			primary_operation_kind: "get"
			capabilities:           ["commerce"]
			route: {method: "GET", path: "/cart"}
			repository: {load_method: "FindByCustomerID", list_method: "ListByCartID", actor_field: "customerID"}
		},
		{
			match: {
				name_prefix: ["CheckoutCart"]
				entity_in:   ["Cart", "Order"]
			}
			primary_operation_kind: "transition"
			capabilities:           ["commerce", "payment"]
			side_effects: [
				{kind: "publish_event", event: "OrderCreated"},
			]
			route: {method: "POST", path: "/cart/checkout"}
		},
		{
			match: {
				name_prefix: ["CreateOrder"]
				entity_in:   ["Order", "OrderItem"]
			}
			primary_operation_kind: "create"
			capabilities:           ["commerce"]
			route: {method: "POST", path: "/orders"}
		},
		{
			match: {
				name_prefix: ["ListOrders", "GetOrder", "CancelOrder"]
				entity_in:   ["Order", "OrderItem"]
			}
			primary_operation_kind: "get"
			capabilities:           ["commerce"]
			route: {path: "/orders"}
			repository: {load_method: "FindByCustomerID", list_method: "ListByCustomerID", actor_field: "customerID"}
		},
		{
			match: {
				name_prefix: ["CreateReview"]
				entity_in:   ["Review"]
			}
			primary_operation_kind: "create"
			capabilities:           ["commerce"]
			route: {method: "POST", path: "/products/{id}/reviews"}
		},
		{
			match: {
				name_prefix: ["ListReviews", "GetReview"]
				entity_in:   ["Review"]
			}
			primary_operation_kind: "get"
			capabilities:           ["commerce"]
			route: {path: "/products/{id}/reviews"}
			repository: {list_method: "ListByProductID", input_field: "productID"}
		},
	]
}

#AuthProfilePackExample: #PackPolicy & {
	name:        "auth_profile"
	description: "Canonical shaping rules for auth and self-profile flows."
	priority:    220

	entity_rules: [
		{
			match: {
				name_in: ["User", "UserProfile", "Session", "RefreshToken"]
			}
			capabilities: ["auth", "profile"]
		},
	]

	operation_rules: [
		{
			match: {
				name_in:   ["RegisterUser", "Register", "Signup", "CreateAccount"]
				entity_in: ["User"]
			}
			primary_operation_kind: "auth"
			capabilities:           ["auth"]
			route: {method: "POST", path: "/auth/register"}
		},
		{
			match: {
				name_in: ["LoginUser", "Login", "Signin"]
			}
			primary_operation_kind: "auth"
			capabilities:           ["auth"]
			route: {method: "POST", path: "/auth/login"}
			repository: {load_method: "FindByEmail", input_field: "email"}
		},
		{
			match: {
				name_in:   ["GetMyProfile", "GetProfile", "GetSelfProfile"]
				entity_in: ["User", "UserProfile"]
			}
			primary_operation_kind: "get"
			capabilities:           ["auth", "profile"]
			route: {method: "GET", path: "/auth/profile"}
			repository: {load_method: "FindByUserID", actor_field: "userID"}
		},
		{
			match: {
				name_in:   ["UpdateMyProfile", "UpdateProfile", "UpdateAvatar", "UpdateAddress"]
				entity_in: ["User", "UserProfile"]
			}
			primary_operation_kind: "update"
			capabilities:           ["auth", "profile"]
			route: {method: "PUT", path: "/auth/profile"}
			repository: {load_method: "FindByUserID", actor_field: "userID"}
		},
	]
}

#MessagingCommunityPackExample: #PackPolicy & {
	name:        "messaging_community"
	description: "Canonical shaping rules for conversations, messages, and community listings."
	priority:    210

	entity_rules: [
		{
			match: {
				name_in: ["Conversation", "ConversationMessage", "CommunityListing"]
			}
			capabilities: ["messaging"]
		},
	]

	operation_rules: [
		{
			match: {
				name_in:   ["SendConversationMessage", "CreateConversationMessage"]
				entity_in: ["ConversationMessage"]
			}
			primary_operation_kind: "message"
			capabilities:           ["messaging"]
			route: {method: "POST", path: "/conversations/{id}/messages"}
			repository: {load_method: "FindByConversationID", input_field: "conversationID"}
		},
		{
			match: {
				name_in:   ["ListConversationMessages", "GetConversationMessages"]
				entity_in: ["ConversationMessage"]
			}
			primary_operation_kind: "list"
			capabilities:           ["messaging"]
			route: {method: "GET", path: "/conversations/{id}/messages"}
			repository: {list_method: "ListByConversationID", input_field: "conversationID"}
		},
		{
			match: {
				name_in:   ["CreateCommunityListing"]
				entity_in: ["CommunityListing"]
			}
			primary_operation_kind: "create"
			capabilities:           ["messaging"]
			route: {method: "POST", path: "/community-listings"}
		},
		{
			match: {
				name_in:   ["ListCommunityListings", "GetCommunityListings"]
				entity_in: ["CommunityListing"]
			}
			primary_operation_kind: "list"
			capabilities:           ["messaging"]
			route: {method: "GET", path: "/community-listings"}
		},
	]
}
