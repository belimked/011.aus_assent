#!/usr/bin/env bash
# Read-only initial-import dry run against Rex sandbox 2057. Stores ids/types/relations only (no PII).
set -uo pipefail
cd "$(dirname "$0")/.."
set -a; . /etc/axleo/rex-sandbox.env; set +a
O=.tmp/rex/imp; mkdir -p $O; : > $O/calls.log
H=(-H 'Content-Type: application/json' -H "X-App-Identifier: $REX_APP_ID")
T=$(cat .tmp/rex/token)
c(){ local r; r=$(curl -sS -m 90 $REX_BASE_URL/v1/rex/$1 "${H[@]}" -H "Authorization: Bearer $T" -d "$2"); echo "$1 $(echo "$r" | jq -r '.error.type // "ok"')" >> $O/calls.log; echo "$r"; }
page(){ # svc body_without_offset jqfilter out
  local svc=$1 body=$2 f=$3 out=$4 off=0 n
  : > $out
  while :; do
    r=$(c $svc/search "$(echo "$body" | jq -c --argjson o $off '. + {offset:$o, limit:100}')")
    n=$(echo "$r" | jq '.result.rows|length')
    [ "$n" = null ] && { echo "ERR $svc @ $off: $(echo "$r" | jq -c .error.message)"; break; }
    echo "$r" | jq -c ".result.rows[] | $f" >> $out
    off=$((off+100)); [ "$n" -lt 100 ] && break
  done
  echo "$svc: $(wc -l < $out) rows"
}
S=$(date +%s)
page properties '{"extra_options":{"extra_fields":["contact_reln_property"]}}' \
  '{id, sub:.adr_suburb_or_town, pc:.adr_postcode, lat:.adr_latitude, lng:.adr_longitude, cat:.property_category.id, owner_user:.system_owner_user.id, etag, relns:[(.related.contact_reln_property // [])[] | {c:.contact.id, t:.reln_type.id, dnc:.do_not_contact}]}' $O/properties.jsonl
page contacts '{"extra_options":{"extra_fields":["related.contact_reln_property","related.contact_reln_listing"]}}' \
  '{id, type, owner_user:.system_owner_user.id, dnd:.is_dnd, letter:.contact_by_letter, etag, rp:[(.related.contact_reln_property // [])[] | {p:.property.id, t:.reln_type.id}], rl:[(.related.contact_reln_listing // [])[] | {l:.listing.id, t:.reln_type.id}]}' $O/contacts.jsonl
page listings '{}' \
  '{id, p:(.property.id // null), state:.system_listing_state, cat:.listing_category.id, a1:(.listing_agent_1.id // null), a2:(.listing_agent_2.id // null), owner_user:(.system_owner_user.id // null)}' $O/listings.jsonl
page account-users '{}' '{id, status:.user_account_status, billed:.user_slot_is_billed}' $O/users.jsonl
echo "elapsed: $(( $(date +%s) - S ))s, calls: $(wc -l < $O/calls.log), errors: $(grep -vc ' ok$' $O/calls.log)"
