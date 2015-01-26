package verror

// TODO(toddw): Replace verror package with verror2.

// Common error IDs used throughout veyron.  Wherever possible try to use these
// common IDs to classify your errors.

const Aborted = ID("v.io/core/veyron2/verror.Aborted") // Operation aborted, e.g. connection closed.

const BadArg = ID("v.io/core/veyron2/verror.BadArg") // Requester specified an invalid argument.

const BadProtocol = ID("v.io/core/veyron2/verror.BadProtocol") // Protocol mismatch, including type or argument errors.

const Exists = ID("v.io/core/veyron2/verror.Exists") // Requested entity already exists.

const Internal = ID("v.io/core/veyron2/verror.Internal") // Internal invariants broken; something is very wrong.

const NoAccess = ID("v.io/core/veyron2/verror.NoAccess") // Requested entity exists, but requester may not access it.

const NoExist = ID("v.io/core/veyron2/verror.NoExist") // Requested entity doesn't exist.

const NoExistOrNoAccess = ID("v.io/core/veyron2/verror.NoExistOrNoAccess") // Requested entity doesn't exist, or requester may not access it.
