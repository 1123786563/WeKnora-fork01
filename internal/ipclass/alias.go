// Pass A compatibility alias for internal/modules/policy/ipclass — zero logic. Deleted by Pass B task B-policy.
package ipclass

import "github.com/Tencent/WeKnora/internal/modules/policy/ipclass"

// Type aliases to the moved package.
type Class = ipclass.Class

// Constants forwarded to the moved package.
const CGNAT = ipclass.CGNAT
const Documentation = ipclass.Documentation
const Invalid = ipclass.Invalid
const LinkLocal = ipclass.LinkLocal
const Loopback = ipclass.Loopback
const Multicast = ipclass.Multicast
const Private = ipclass.Private
const Public = ipclass.Public
const Reserved = ipclass.Reserved
const SiteLocalIPv6 = ipclass.SiteLocalIPv6
const Translated = ipclass.Translated
const Unspecified = ipclass.Unspecified

// Variables and functions forwarded to the moved package.
var Classify = ipclass.Classify
var IsPublic = ipclass.IsPublic
