// Ported from adobe/structured-data-validator (src/types/*.js). Copyright 2025 Adobe.
// This file is licensed to you under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License. You may obtain a copy
// of the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR REPRESENTATIONS
// OF ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Modified by LaFactory (Go port, scope and added Google rules): see THIRD_PARTY_NOTICES.

package audit

import (
	"strings"

	"github.com/astappiev/microdata"
)

// sdStep is one level of the path from the root item: the type of the item and
// the property of its parent that holds it.
type sdStep struct {
	Type     string
	Property string
}

type ruleCtx struct {
	it   *microdata.Item
	typ  string
	path []sdStep
	out  *[]SDIssue
}

func (r *ruleCtx) add(field, kind, severity string) {
	*r.out = append(*r.out, SDIssue{Type: r.typ, Field: field, Kind: kind, Severity: severity})
}

func (r *ruleCtx) get(field string) (interface{}, bool) { return value(r.it, field) }

func (r *ruleCtx) has(field string) bool { return present(r.get(field)) }

// required: missing or of the wrong type is an error (base.js required()).
func (r *ruleCtx) required(field string, check func(interface{}) bool) bool {
	v, ok := r.get(field)
	if !present(v, ok) {
		r.add(field, "missing", "error")
		return false
	}
	if check != nil && !check(v) {
		r.add(field, "invalid", "error")
		return false
	}
	return true
}

// recommended: missing or of the wrong type is a warning (base.js recommended()).
func (r *ruleCtx) recommended(field string, check func(interface{}) bool) {
	v, ok := r.get(field)
	if !present(v, ok) {
		r.add(field, "missing", "warning")
		return
	}
	if check != nil && !check(v) {
		r.add(field, "invalid", "warning")
	}
}

// valid tells whether a field is set and passes the check, adding nothing.
func (r *ruleCtx) valid(field string, check func(interface{}) bool) bool {
	v, ok := r.get(field)
	return present(v, ok) && (check == nil || check(v))
}

// oneOf is base.js or(): one of the fields must be set and valid.
func (r *ruleCtx) oneOf(severity string, check func(interface{}) bool, fields ...string) {
	for _, f := range fields {
		if r.valid(f, check) {
			return
		}
	}
	kind := "missing"
	for _, f := range fields {
		if r.has(f) {
			kind = "invalid"
		}
	}
	r.add(strings.Join(fields, "|"), kind, severity)
}

func (r *ruleCtx) isRoot() bool { return len(r.path) == 1 }

func (r *ruleCtx) parent() sdStep {
	if len(r.path) < 2 {
		return sdStep{}
	}
	return r.path[len(r.path)-2]
}

func (r *ruleCtx) property() string { return r.path[len(r.path)-1].Property }

// localBusinessTypes are the LocalBusiness subtypes small businesses use most;
// they follow the LocalBusiness rules of Google's documentation.
var localBusinessTypes = map[string]bool{
	"LocalBusiness": true, "Store": true, "ProfessionalService": true, "HomeAndConstructionBusiness": true,
	"GeneralContractor": true, "Electrician": true, "Plumber": true, "HVACBusiness": true, "RoofingContractor": true,
	"AutomotiveBusiness": true, "FoodEstablishment": true, "Restaurant": true, "LodgingBusiness": true, "Hotel": true,
	"HealthAndBeautyBusiness": true, "MedicalBusiness": true, "LegalService": true, "FinancialService": true,
	"RealEstateAgent": true, "TravelAgency": true, "SportsActivityLocation": true, "EntertainmentBusiness": true,
	"ShoppingCenter": true, "HomeGoodsStore": true, "HardwareStore": true, "GardenStore": true, "ClothingStore": true,
	"ElectronicsStore": true, "FurnitureStore": true, "AutoPartsStore": true, "SportingGoodsStore": true,
}

var organizationTypes = map[string]bool{
	"Organization": true, "Corporation": true, "OnlineStore": true, "OnlineBusiness": true, "NGO": true,
}

// validateItem applies the rules of every type of an item, then walks into its
// nested items (validator.js #validateSubtree).
func validateItem(it *microdata.Item, path []sdStep, out *[]SDIssue) {
	if it == nil || len(path) > 8 {
		return
	}
	last := path[len(path)-1]
	for _, t := range itemTypes(it) {
		p := append(append([]sdStep{}, path[:len(path)-1]...), sdStep{Type: t, Property: last.Property})
		applyRules(&ruleCtx{it: it, typ: t, path: p, out: out})
	}
	here := append(append([]sdStep{}, path[:len(path)-1]...), sdStep{Type: mainType(it), Property: last.Property})
	for prop, vals := range it.Properties {
		if strings.HasPrefix(prop, "@") {
			continue
		}
		for _, v := range vals {
			child, ok := v.(*microdata.Item)
			if !ok || len(child.Types) == 0 {
				continue
			}
			validateItem(child, append(append([]sdStep{}, here...), sdStep{Property: prop}), out)
		}
	}
}

func applyRules(r *ruleCtx) {
	switch {
	case r.typ == "Product":
		productRules(r)
	case r.typ == "Offer":
		offerRules(r)
	case r.typ == "AggregateOffer":
		r.required("lowPrice", isNumber)
		r.required("priceCurrency", isCurrency)
		r.recommended("highPrice", isNumber)
		r.recommended("offerCount", isNumber)
	case r.typ == "AggregateRating":
		ratingRules(r)
		r.oneOf("error", isNumber, "ratingCount", "reviewCount")
		if r.isRoot() {
			r.required("itemReviewed", nil)
			r.required("itemReviewed.name", nil)
		}
	case r.typ == "Rating":
		ratingRules(r)
	case r.typ == "Review":
		r.required("author", nil)
		r.required("reviewRating", nil)
		r.recommended("datePublished", isDate)
		if r.isRoot() {
			r.required("itemReviewed", nil)
			r.required("itemReviewed.name", nil)
		}
	case r.typ == "Brand" || r.typ == "Person":
		r.required("name", nil)
	case r.typ == "BreadcrumbList":
		breadcrumbRules(r)
	case r.typ == "ListItem":
		if r.parent().Type == "BreadcrumbList" {
			r.oneOf("error", isString, "name", "item.name")
			r.required("position", nil)
		}
	case r.typ == "OfferShippingDetails":
		r.required("deliveryTime", nil)
		r.required("shippingDestination", nil)
		if r.required("shippingRate", nil) {
			r.required("shippingRate.currency", nil)
			r.oneOf("error", nil, "shippingRate.value", "shippingRate.maxValue")
		}
	case r.typ == "MerchantReturnPolicy":
		returnPolicyRules(r)
	case localBusinessTypes[r.typ]:
		r.required("name", nil)
		r.required("address", nil)
		r.recommended("telephone", nil)
		r.recommended("url", nil)
		r.recommended("openingHoursSpecification", nil)
	case organizationTypes[r.typ]:
		// Google's Organization documentation has no required property.
		r.recommended("name", nil)
		if r.isRoot() {
			r.recommended("url", nil)
			r.recommended("logo", nil)
		}
	case r.typ == "PostalAddress":
		if localBusinessTypes[r.parent().Type] && r.property() == "address" {
			r.recommended("streetAddress", nil)
			r.recommended("addressLocality", nil)
			r.recommended("postalCode", nil)
			r.recommended("addressCountry", nil)
		}
	case r.typ == "FAQPage":
		r.required("mainEntity", nil)
	case r.typ == "Question":
		if r.parent().Type == "FAQPage" {
			r.required("name", nil)
			if r.required("acceptedAnswer", nil) {
				r.required("acceptedAnswer.text", nil)
			}
		}
	}
}

func isString(v interface{}) bool { _, ok := v.(string); return ok }

// productRules: Product.js and ProductMerchant.js (merchant listing). The
// apparel and variant fields of merchant listings are left out of the audit.
func productRules(r *ruleCtx) {
	r.required("name", nil)
	hasRating, hasOffers, hasReview := r.has("aggregateRating"), r.has("offers"), r.has("review")
	if !hasRating && !hasOffers && !hasReview {
		r.add("aggregateRating|offers|review", "missing", "error")
	}
	if hasOffers && !hasRating {
		r.add("aggregateRating", "missing", "warning")
	}
	if hasOffers && !hasReview {
		r.add("review", "missing", "warning")
	}
	r.required("image", nil)
	if hasRating || hasReview {
		// Without any of the three, the error above already names offers.
		r.required("offers", nil)
	}
	r.recommended("brand", nil)
	r.recommended("description", isString)
	r.recommended("sku", isString)
	productGTIN(r)
}

var gtinFields = []string{"gtin", "gtin8", "gtin12", "gtin13", "gtin14", "isbn"}

// productGTIN: one GTIN on the product, or on every offer (validateGtin).
func productGTIN(r *ruleCtx) {
	hasGTIN := func(it *microdata.Item) bool {
		for _, f := range gtinFields {
			if v, ok := value(it, f); present(v, ok) {
				return true
			}
		}
		return false
	}
	if hasGTIN(r.it) {
		return
	}
	offers := values(r.it, "offers")
	all := len(offers) > 0
	for _, o := range offers {
		oi, ok := o.(*microdata.Item)
		if !ok || !hasGTIN(oi) {
			all = false
		}
	}
	if !all {
		r.add("gtin", "missing", "warning")
	}
}

// offerRules: Offer.js, applied when the offer belongs to a product, plus the
// delivery and returns fields Google recommends for merchant listings.
func offerRules(r *ruleCtx) {
	if r.parent().Type != "Product" {
		return
	}
	if !r.valid("price", isNumber) && !r.valid("priceSpecification.price", isNumber) {
		kind := "missing"
		if r.has("price") || r.has("priceSpecification.price") {
			kind = "invalid"
		}
		r.add("price", kind, "error")
	}
	r.recommended("availability", nil)
	if !r.valid("priceCurrency", isCurrency) && !r.valid("priceSpecification.priceCurrency", isCurrency) {
		kind := "missing"
		if r.has("priceCurrency") || r.has("priceSpecification.priceCurrency") {
			kind = "invalid"
		}
		r.add("priceCurrency", kind, "warning")
	}
	r.recommended("priceValidUntil", isDate)
	r.recommended("shippingDetails", nil)
	r.recommended("hasMerchantReturnPolicy", nil)
}

// ratingRules: Rating.js without bestRating and worstRating, which Google's
// own validator does not ask for (it assumes 1 to 5).
func ratingRules(r *ruleCtx) {
	if !r.required("ratingValue", nil) {
		return
	}
	v, _ := r.get("ratingValue")
	if _, isStr := v.(string); isStr {
		// Google reads a text value ("4,5", "90 %") without a range.
		return
	}
	n, ok := toNumber(v)
	if !ok {
		return
	}
	from, to := 0.0, 5.0
	if w, ok := r.get("worstRating"); ok {
		if f, ok := toNumber(w); ok {
			from = f
		}
	}
	if b, ok := r.get("bestRating"); ok {
		if f, ok := toNumber(b); ok {
			to = f
		}
	}
	if n < from || n > to {
		r.add("ratingValue", "range", "error")
	}
}

// breadcrumbRules: BreadcrumbList.js. Every item but the last needs a URL.
func breadcrumbRules(r *ruleCtx) {
	items := values(r.it, "itemListElement")
	if len(items) == 0 {
		r.add("itemListElement", "missing", "error")
		return
	}
	if len(items) < 2 {
		r.add("itemListElement", "few_items", "warning")
	}
	last, maxPos, allNumbers := -1, 0.0, true
	for i, v := range items {
		li, ok := v.(*microdata.Item)
		if !ok {
			allNumbers = false
			continue
		}
		p, ok := value(li, "position")
		n, isNum := toNumber(p)
		if !ok || !isNum {
			allNumbers = false
			continue
		}
		if last < 0 || n > maxPos {
			last, maxPos = i, n
		}
	}
	for i, v := range items {
		li, ok := v.(*microdata.Item)
		if !ok {
			continue
		}
		var target interface{}
		if it, ok := value(li, "item"); ok {
			if nested, isItem := it.(*microdata.Item); isItem {
				if id, ok := value(nested, "@id"); ok {
					target = id
				} else if u, ok := value(nested, "url"); ok {
					target = u
				}
			} else {
				target = it
			}
		}
		s, _ := target.(string)
		s = strings.TrimSpace(s)
		switch {
		case s == "" && allNumbers && i == last:
		case s == "":
			r.add("itemListElement.item", "missing", "warning")
		case strings.HasPrefix(s, "data:"):
			r.add("itemListElement.item", "invalid", "warning")
		}
	}
}

// returnPolicyRules: MerchantReturnPolicy.js.
func returnPolicyRules(r *ruleCtx) {
	inOrg := r.property() == "hasMerchantReturnPolicy" && (organizationTypes[r.parent().Type] || localBusinessTypes[r.parent().Type])
	if !(inOrg && r.has("merchantReturnLink")) && !(r.has("applicableCountry") && r.has("returnPolicyCategory")) {
		r.add("applicableCountry|returnPolicyCategory", "missing", "error")
	}
	c, _ := r.get("returnPolicyCategory")
	cat, _ := c.(string)
	if strings.Contains(cat, "MerchantReturnFiniteReturnWindow") || strings.Contains(cat, "MerchantReturnUnlimitedWindow") {
		if strings.Contains(cat, "MerchantReturnFiniteReturnWindow") {
			r.recommended("merchantReturnDays", isNumber)
		}
		r.recommended("returnFees", nil)
		r.recommended("returnMethod", nil)
	}
}
