package schema

// Standard AI Patterns for ANG
// These schemas help AI agents use consistent structures

#StandardID: {
    id: string @ui(hidden, disabled)
}

#Timestamps: {
    created_at: string @ui(hidden, disabled)
    updated_at: string @ui(hidden, disabled)
}

#SoftDelete: {
    deleted_at?: string @ui(hidden)
}

// Full Audit pattern for sensitive entities
#AuditTrail: {
    #Timestamps
    created_by: string
    updated_by?: string
}

// Common Result pattern for actions
#OperationResult: {
    ok: bool
    message?: string
}
