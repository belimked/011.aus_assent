// Prototype seed() and migrate() snapshot. Regenerate with node tools/extract-i18n.mjs.
import type { AppState } from './types';
export const TODAY = '2026-11-07';
export const seed = (): AppState => JSON.parse(JSON.stringify({
  "v": 8,
  "auth": null,
  "audit": [],
  "leads": [],
  "zones": [
    {
      "id": "z1",
      "name": "Gager West",
      "pts": [
        [
          -27.577191000000003,
          153.05494000000002
        ],
        [
          -27.577191000000003,
          153.056008
        ],
        [
          -27.576597999999997,
          153.056008
        ],
        [
          -27.576597999999997,
          153.05494000000002
        ]
      ]
    }
  ],
  "referrals": [
    {
      "id": "rf1",
      "type": "broker",
      "to": "Lisa Tran (Axleo Finance)",
      "contactId": "c6",
      "propertyId": "p2",
      "d": "2026-11-01",
      "status": "sent"
    }
  ],
  "syncq": [],
  "rexDown": false,
  "contacts": [
    {
      "id": "c1",
      "first": "Mary",
      "last": "Chen",
      "mobile": "0412 555 019",
      "email": "mary.chen@example.com",
      "roles": [
        "owner"
      ],
      "owner": "Owen Chen",
      "tags": [
        "Seller lead",
        "Warm",
        "Downsizing",
        "Friendly",
        "Cantonese speaking",
        "Spring 2027"
      ],
      "reason": "Downsizing",
      "flags": {},
      "optedOut": false,
      "rels": [
        {
          "type": "Partner of",
          "name": "John Chen"
        },
        {
          "type": "Co-owner with",
          "name": "John Chen"
        }
      ],
      "updated": {
        "by": "Owen Chen",
        "d": "2026-08-14"
      },
      "notes": [
        {
          "d": "2026-08-14",
          "a": "Owen Chen",
          "t": "Spoke at door. Kids moved out, thinking of downsizing next year."
        },
        {
          "d": "2025-11-02",
          "a": "Natalie Wong",
          "t": "Market update sent. Happy to stay in touch."
        }
      ],
      "buyerProfile": null,
      "history": [],
      "dncr": {
        "d": "2026-10-28",
        "listed": false
      },
      "consent": {
        "email": 1,
        "sms": 1,
        "phone": 1,
        "letter": 1
      }
    },
    {
      "id": "c2",
      "first": "John",
      "last": "Lee",
      "mobile": "0433 221 877",
      "email": "john.lee@example.com",
      "roles": [
        "buyer",
        "owner"
      ],
      "owner": "Owen Chen",
      "tags": [
        "Investor",
        "Warm"
      ],
      "reason": "",
      "flags": {},
      "optedOut": false,
      "rels": [],
      "updated": {
        "by": "Owen Chen",
        "d": "2026-10-25"
      },
      "notes": [
        {
          "d": "2026-10-25",
          "a": "Owen Chen",
          "t": "Attended OFI 22 Lister St. Owns in Runcorn, may sell next year."
        }
      ],
      "buyerProfile": {
        "price": "$900k–1.1m",
        "suburbs": "Sunnybank, Runcorn",
        "beds": 4,
        "finance": "Pre-approved",
        "timing": "3 months",
        "must": [
          "Single level"
        ],
        "occupy": "Live in"
      },
      "history": [
        {
          "d": "2026-10-25",
          "t": "OFI 22 Lister St"
        }
      ],
      "dncr": {
        "d": "2026-10-28",
        "listed": false
      },
      "consent": {
        "email": 1,
        "sms": 1,
        "phone": 1,
        "letter": 1
      }
    },
    {
      "id": "c3",
      "first": "David",
      "last": "Wu",
      "mobile": "0401 777 302",
      "email": "",
      "roles": [
        "owner"
      ],
      "owner": "Owen Chen",
      "tags": [
        "Phone provided"
      ],
      "reason": "",
      "flags": {},
      "optedOut": true,
      "consent": {
        "email": 1,
        "sms": 0,
        "phone": 0,
        "letter": 1
      },
      "prefLog": {
        "d": "2026-11-06",
        "by": "Natalie Wong",
        "t": "Text: replied STOP 4 Nov · Call: asked by phone 6 Nov"
      },
      "dncr": {
        "d": "2026-09-01",
        "listed": false
      },
      "rels": [],
      "updated": {
        "by": "Owen Chen",
        "d": "2024-06-12"
      },
      "notes": [
        {
          "d": "2024-06-12",
          "a": "Owen Chen",
          "t": "Purchased 8 Gager St through us. Settled."
        }
      ],
      "buyerProfile": null,
      "history": [
        {
          "d": "2024-05-04",
          "t": "Contract 8 Gager St (buyer)"
        }
      ]
    },
    {
      "id": "c4",
      "first": "Helen",
      "last": "Papas",
      "mobile": "0455 109 884",
      "email": "",
      "roles": [
        "owner"
      ],
      "owner": "Natalie Wong",
      "tags": [
        "Do not contact",
        "Not interested"
      ],
      "reason": "",
      "flags": {
        "dnc": true
      },
      "optedOut": true,
      "rels": [],
      "updated": {
        "by": "Natalie Wong",
        "d": "2026-09-30"
      },
      "notes": [
        {
          "d": "2026-09-30",
          "a": "Natalie Wong",
          "t": "Asked us not to contact again. Replied STOP to market update."
        }
      ],
      "buyerProfile": null,
      "history": [],
      "dncr": {
        "d": "2026-10-28",
        "listed": false
      },
      "consent": {
        "email": 0,
        "sms": 0,
        "phone": 0,
        "letter": 0
      }
    },
    {
      "id": "c5",
      "first": "Sam",
      "last": "Nguyen",
      "mobile": "0420 388 115",
      "email": "sam.n@example.com",
      "roles": [
        "tenant"
      ],
      "owner": "Owen Chen",
      "tags": [],
      "reason": "",
      "flags": {},
      "optedOut": false,
      "rels": [
        {
          "type": "Tenant of",
          "name": "Y Zhang"
        }
      ],
      "updated": {
        "by": "Owen Chen",
        "d": "2026-05-02"
      },
      "notes": [],
      "buyerProfile": null,
      "history": [],
      "dncr": {
        "d": "2026-10-28",
        "listed": false
      },
      "consent": {
        "email": 1,
        "sms": 1,
        "phone": 1,
        "letter": 1
      }
    },
    {
      "id": "c6",
      "first": "Grace",
      "last": "Ho",
      "mobile": "0422 650 777",
      "email": "grace.ho@example.com",
      "roles": [
        "buyer"
      ],
      "owner": "Natalie Wong",
      "tags": [
        "Hot"
      ],
      "reason": "",
      "flags": {},
      "optedOut": false,
      "rels": [],
      "updated": {
        "by": "Natalie Wong",
        "d": "2026-11-01"
      },
      "notes": [
        {
          "d": "2026-11-01",
          "a": "Natalie Wong",
          "t": "Looking for 4 bed in Sunnybank or Runcorn, up to $1.2m. Pre-approved."
        }
      ],
      "buyerProfile": {
        "price": "$1.0–1.2m",
        "suburbs": "Sunnybank, Runcorn",
        "beds": 4,
        "finance": "Pre-approved",
        "timing": "ASAP",
        "must": [
          "Pool"
        ],
        "occupy": "Live in"
      },
      "history": [],
      "dncr": {
        "d": "2026-10-28",
        "listed": false
      },
      "consent": {
        "email": 1,
        "sms": 1,
        "phone": 1,
        "letter": 1
      }
    },
    {
      "id": "c8",
      "first": "John",
      "last": "Chen",
      "mobile": "0412 555 020",
      "email": "mary.chen@example.com",
      "roles": [
        "owner"
      ],
      "owner": "Owen Chen",
      "tags": [
        "Friendly"
      ],
      "reason": "",
      "flags": {},
      "optedOut": false,
      "consent": {
        "email": 1,
        "sms": 1,
        "phone": 1,
        "letter": 0
      },
      "rels": [
        {
          "type": "Partner of",
          "name": "Mary Chen"
        }
      ],
      "updated": {
        "by": "Owen Chen",
        "d": "2026-08-14"
      },
      "notes": [
        {
          "d": "2026-08-14",
          "a": "Owen Chen",
          "t": "Mary's partner. Shares the household email."
        }
      ],
      "buyerProfile": null,
      "history": [],
      "dncr": {
        "d": "2026-10-28",
        "listed": false
      }
    },
    {
      "id": "c9",
      "first": "Mary",
      "last": "Chen",
      "mobile": "0412555019",
      "email": "",
      "roles": [
        "owner"
      ],
      "owner": "Owen Chen",
      "tags": [
        "Market update"
      ],
      "reason": "",
      "flags": {},
      "optedOut": false,
      "rels": [],
      "updated": {
        "by": "Owen Chen",
        "d": "2025-06-02"
      },
      "notes": [
        {
          "d": "2025-06-02",
          "a": "Owen Chen",
          "t": "Wants a market update once a year. (Entered by hand, before the duplicate check.)"
        }
      ],
      "buyerProfile": null,
      "history": [],
      "dncr": {
        "d": "2026-10-28",
        "listed": false
      },
      "consent": {
        "email": 1,
        "sms": 1,
        "phone": 1,
        "letter": 1
      }
    },
    {
      "id": "c7",
      "first": "Peter",
      "last": "Kowalski",
      "mobile": "0418 332 760",
      "email": "",
      "roles": [
        "owner"
      ],
      "owner": "Natalie Wong",
      "tags": [
        "Cold",
        "Market update"
      ],
      "reason": "",
      "flags": {},
      "optedOut": false,
      "dncr": {
        "d": "2026-11-01",
        "listed": true
      },
      "rels": [],
      "updated": {
        "by": "Natalie Wong",
        "d": "2026-03-12"
      },
      "notes": [
        {
          "d": "2026-03-12",
          "a": "Natalie Wong",
          "t": "Retired, happy with a market update once a year."
        }
      ],
      "buyerProfile": null,
      "history": [],
      "consent": {
        "email": 1,
        "sms": 1,
        "phone": 1,
        "letter": 1
      }
    }
  ],
  "properties": [
    {
      "id": "p1",
      "no": "2",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [],
      "pfOwner": "K & L Tran",
      "lastSale": "2009-03",
      "beds": 4,
      "baths": 2,
      "cars": 2,
      "land": "612 m²",
      "lot": "Lot 12 RP 123456",
      "flags": {},
      "visits": [],
      "ll": [
        -27.576848,
        153.05519
      ]
    },
    {
      "id": "p2",
      "no": "4",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [],
      "pfOwner": "R Singh",
      "lastSale": "2015-07",
      "beds": 4,
      "baths": 2,
      "cars": 2,
      "land": "640 m²",
      "lot": "Lot 13 RP 123456",
      "flags": {},
      "visits": [
        {
          "d": "2026-10-28",
          "o": "not_home"
        }
      ],
      "ll": [
        -27.576879,
        153.05538
      ]
    },
    {
      "id": "p3",
      "no": "6",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [
        "c4"
      ],
      "pfOwner": "H Papas",
      "lastSale": "2001-11",
      "beds": 3,
      "baths": 1,
      "cars": 1,
      "land": "605 m²",
      "lot": "Lot 14 RP 123456",
      "flags": {
        "dnc": true
      },
      "visits": [
        {
          "d": "2026-09-30",
          "o": "not_int"
        }
      ],
      "ll": [
        -27.57691,
        153.055569
      ]
    },
    {
      "id": "p4",
      "no": "8",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [
        "c3"
      ],
      "pfOwner": "D Wu",
      "lastSale": "2024-06",
      "beds": 4,
      "baths": 2,
      "cars": 2,
      "land": "620 m²",
      "lot": "Lot 15 RP 123456",
      "pinned": true,
      "flags": {},
      "visits": [],
      "ll": [
        -27.576941,
        153.055758
      ]
    },
    {
      "id": "p5",
      "no": "10",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [],
      "pfOwner": "",
      "lastSale": "",
      "beds": 3,
      "baths": 1,
      "cars": 1,
      "land": "600 m²",
      "lot": "",
      "flags": {},
      "visits": [],
      "pending": true,
      "ll": [
        -27.576972,
        153.055948
      ]
    },
    {
      "id": "p6",
      "no": "12",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [
        "c1"
      ],
      "pfOwner": "M Chen & J Chen",
      "lastSale": "2014-02",
      "beds": 5,
      "baths": 3,
      "cars": 2,
      "land": "810 m²",
      "lot": "Lot 17 RP 123456",
      "flags": {},
      "visits": [
        {
          "d": "2026-08-14",
          "o": "spoke"
        }
      ],
      "pinned": false,
      "ll": [
        -27.577003,
        153.056137
      ]
    },
    {
      "id": "p7",
      "no": "14",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [
        "c7"
      ],
      "pfOwner": "P Kowalski",
      "lastSale": "2011-09",
      "beds": 3,
      "baths": 2,
      "cars": 2,
      "land": "630 m²",
      "lot": "Lot 18 RP 123456",
      "flags": {},
      "visits": [],
      "otherAgent": true,
      "ll": [
        -27.577034,
        153.056327
      ]
    },
    {
      "id": "p8",
      "no": "16",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [],
      "pfOwner": "Y Zhang",
      "lastSale": "2019-05",
      "beds": 4,
      "baths": 2,
      "cars": 2,
      "land": "615 m²",
      "lot": "Lot 19 RP 123456",
      "flags": {},
      "visits": [],
      "appraisal": "2026-11-10 16:00",
      "ll": [
        -27.577065,
        153.056516
      ]
    },
    {
      "id": "p11",
      "no": "1",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [],
      "pfOwner": "A Rossi",
      "lastSale": "1998-02",
      "beds": 3,
      "baths": 1,
      "cars": 1,
      "land": "700 m²",
      "lot": "Lot 2 RP 123450",
      "flags": {},
      "visits": [],
      "otherListing": true,
      "ll": [
        -27.577272,
        153.055102
      ]
    },
    {
      "id": "p12",
      "no": "5",
      "street": "Gager St",
      "suburb": "Sunnybank",
      "ownerIds": [],
      "pfOwner": "W Park",
      "lastSale": "2021-10",
      "beds": 4,
      "baths": 2,
      "cars": 2,
      "land": "650 m²",
      "lot": "Lot 4 RP 123450",
      "flags": {},
      "visits": [],
      "ll": [
        -27.577334,
        153.055481
      ]
    },
    {
      "id": "p9",
      "no": "22",
      "street": "Lister St",
      "suburb": "Sunnybank",
      "ownerIds": [],
      "pfOwner": "B & C Moore",
      "lastSale": "2006-10",
      "beds": 4,
      "baths": 2,
      "cars": 2,
      "land": "720 m²",
      "lot": "Lot 7 RP 118822",
      "flags": {},
      "visits": [],
      "ll": [
        -27.578622,
        153.056054
      ]
    },
    {
      "id": "p10",
      "no": "7",
      "street": "Hill St",
      "suburb": "Runcorn",
      "ownerIds": [
        "c2"
      ],
      "pfOwner": "J Lee",
      "lastSale": "2018-04",
      "beds": 4,
      "baths": 2,
      "cars": 2,
      "land": "600 m²",
      "lot": "Lot 31 SP 220011",
      "flags": {},
      "visits": [],
      "ll": [
        -27.5941,
        153.0782
      ]
    }
  ],
  "listings": [
    {
      "id": "l1",
      "propertyId": "p2",
      "agent": "Owen Chen",
      "price": "Offers over $1.05m",
      "status": "listed",
      "ofi": [
        {
          "d": "2026-11-07",
          "time": "10:00–10:30",
          "attendees": []
        }
      ],
      "bpReport": true,
      "form2": true
    },
    {
      "id": "l2",
      "propertyId": "p9",
      "agent": "Natalie Wong",
      "price": "Auction 22 Nov",
      "status": "listed",
      "ofi": [
        {
          "d": "2026-11-07",
          "time": "11:00–11:30",
          "attendees": [
            {
              "contactId": "c2",
              "d": "2026-10-25",
              "req": {
                "bp": true
              },
              "ps": true,
              "note": "Owns in Runcorn"
            }
          ]
        },
        {
          "d": "2026-10-25",
          "time": "11:00–11:30",
          "attendees": [
            {
              "contactId": "c2",
              "d": "2026-10-25",
              "req": {
                "bp": true
              },
              "ps": true,
              "note": ""
            }
          ]
        }
      ],
      "bpReport": false,
      "form2": true
    },
    {
      "id": "l3",
      "propertyId": "p4",
      "agent": "Owen Chen",
      "price": "$1,180,000",
      "status": "sold",
      "ofi": [],
      "bpReport": false,
      "form2": false
    }
  ],
  "contracts": [
    {
      "id": "k1",
      "listingId": "l3",
      "buyerId": "c3",
      "sellerIds": [],
      "sellerName": "T & M Russo",
      "d": "2024-05-04",
      "price": "$1,180,000",
      "status": "settled"
    }
  ],
  "sales": [
    {
      "id": "s1",
      "propertyId": "p9",
      "cat": "unconditional",
      "seller": "B & C Moore",
      "buyer": "Grace Ho",
      "price": 1180000,
      "gci": 29500,
      "settle": "2026-12-05",
      "reminders": []
    },
    {
      "id": "s2",
      "propertyId": "p10",
      "cat": "unconditional",
      "seller": "J Lee",
      "buyer": "S & T Patel",
      "price": 965000,
      "gci": 24100,
      "settle": "2026-11-28",
      "reminders": []
    },
    {
      "id": "s3",
      "propertyId": "p4",
      "cat": "settling",
      "seller": "T & M Russo",
      "buyer": "David Wu",
      "price": 1180000,
      "gci": 29500,
      "settle": "2026-11-12",
      "reminders": [
        {
          "type": "Final inspection",
          "d": "2026-11-11",
          "time": "10:00"
        }
      ]
    },
    {
      "id": "s4",
      "propertyId": "p8",
      "cat": "appraisal",
      "seller": "Y Zhang",
      "buyer": "",
      "price": 0,
      "gci": 0,
      "settle": "",
      "when": "2026-11-10 16:00",
      "reminders": []
    },
    {
      "id": "s5",
      "propertyId": "p12",
      "cat": "appraisal",
      "seller": "W Park",
      "buyer": "",
      "price": 0,
      "gci": 0,
      "settle": "",
      "when": "2026-11-13 11:00",
      "reminders": []
    },
    {
      "id": "s6",
      "propertyId": "p2",
      "cat": "listed",
      "seller": "R Singh",
      "buyer": "",
      "price": 1050000,
      "gci": 26250,
      "settle": "",
      "reminders": []
    },
    {
      "id": "s7",
      "propertyId": "p9",
      "cat": "listed",
      "seller": "B & C Moore",
      "buyer": "",
      "price": 1150000,
      "gci": 28750,
      "settle": "",
      "reminders": []
    }
  ],
  "enquiries": [
    {
      "id": "e1",
      "source": "realestate.com.au",
      "d": "2026-11-07 08:12",
      "type": "buyer",
      "status": "new",
      "raw": "Hi, I'm interested in 4 Gager St Sunnybank. Could I get the building and pest report and inspection times? Sarah Lim 0433 908 112 sarah.lim@example.com",
      "parsed": {
        "first": "Sarah",
        "last": "Lim",
        "mobile": "0433 908 112",
        "email": "sarah.lim@example.com",
        "propertyId": "p2",
        "want": "B&P report + inspection times"
      }
    },
    {
      "id": "e2",
      "source": "Phone",
      "d": "2026-11-07 09:40",
      "type": "seller",
      "status": "new",
      "raw": "Called the office. Peter Kowalski, 14 Gager St. Thinking of selling early next year, wants an appraisal. 0418 332 760.",
      "parsed": {
        "first": "Peter",
        "last": "Kowalski",
        "mobile": "0418 332 760",
        "email": "",
        "propertyId": "p7",
        "want": "Appraisal, selling early 2027"
      }
    },
    {
      "id": "e3",
      "source": "Domain",
      "d": "2026-11-06 17:55",
      "type": "buyer",
      "status": "new",
      "raw": "John Lee here, saw 4 Gager St listed, is Saturday's open home still on? 0433 221 877",
      "parsed": {
        "first": "John",
        "last": "Lee",
        "mobile": "0433 221 877",
        "email": "",
        "propertyId": "p2",
        "want": "Open home time"
      }
    },
    {
      "id": "e5",
      "source": "eplace.com.au",
      "d": "2026-11-07 10:05",
      "type": "seller",
      "status": "new",
      "raw": "Web form: J Chen, mary.chen@example.com, 12 Gager St Sunnybank. What would our place sell for now?",
      "parsed": {
        "first": "J",
        "last": "Chen",
        "mobile": "",
        "email": "mary.chen@example.com",
        "propertyId": "p6",
        "want": "Appraisal / price indication"
      }
    },
    {
      "id": "e4",
      "source": "eplace.com.au",
      "d": "2026-11-04 12:20",
      "type": "seller",
      "status": "filed",
      "raw": "Web form: Mary Chen, 12 Gager St, request market update.",
      "parsed": {
        "first": "Mary",
        "last": "Chen",
        "mobile": "0412 555 019",
        "email": "mary.chen@example.com",
        "propertyId": "p6",
        "want": "Market update"
      }
    }
  ],
  "todos": [
    {
      "id": "t1",
      "d": "2026-11-05",
      "t": "Call Sarah Lim re 4 Gager St B&P",
      "who": "Owen Chen",
      "kind": "call",
      "by": "Owen Chen"
    },
    {
      "id": "t2",
      "d": "2026-11-07",
      "time": "12:30",
      "t": "Feedback call · John Lee · 22 Lister St OFI",
      "who": "Natalie Wong",
      "kind": "call",
      "by": "Natalie Wong"
    },
    {
      "id": "t3",
      "d": "2026-11-07",
      "time": "16:00",
      "t": "Appraisal · 16 Gager St · Y Zhang",
      "who": "Owen Chen",
      "kind": "appt",
      "by": "Owen Chen"
    },
    {
      "id": "t4",
      "d": "2026-11-11",
      "time": "10:00",
      "t": "Final inspection · 8 Gager St",
      "who": "Owen Chen",
      "kind": "appt",
      "by": "Owen Chen"
    },
    {
      "id": "t5",
      "d": "2026-11-12",
      "t": "Hot seller · Mary Chen · send comparable sales",
      "who": "Owen Chen",
      "kind": "hot",
      "by": "Owen Chen"
    },
    {
      "id": "t6",
      "d": "2027-01-15",
      "t": "Follow up Mary Chen · 12 Gager St (next year)",
      "who": "Owen Chen",
      "kind": "call",
      "by": "Owen Chen"
    },
    {
      "id": "t7",
      "d": "2026-11-07",
      "time": "16:00",
      "t": "Drop keys to seller",
      "who": "Owen Chen",
      "by": "Natalie Wong",
      "kind": "appt",
      "cid": "c3",
      "propId": "p4"
    },
    {
      "id": "t8",
      "d": "2026-11-12",
      "time": "16:00",
      "t": "Send market update and call",
      "who": "Natalie Wong",
      "by": "Owen Chen",
      "kind": "call",
      "cid": "c1",
      "propId": "p6"
    }
  ],
  "approvals": [
    {
      "id": "a1",
      "kind": "voice",
      "t": "“Spoke to Mary at 12 Gager St, kids moved out, might sell next year…”",
      "sub": "Mary Chen · 4 fields · 1 tag"
    },
    {
      "id": "a2",
      "kind": "sale",
      "t": "6 Gager St sold 2026-11-03 · $1.21m (Pricefinder)",
      "sub": "Helen Papas → previous owner?",
      "propertyId": "p3"
    },
    {
      "id": "a3",
      "kind": "lead",
      "t": "Domain lead · Alex Park · 0407 118 ••••",
      "sub": "No match in Rex · new contact?"
    }
  ],
  "partners": {
    "broker": [
      "Lisa Tran (Axleo Finance)",
      "Mark Ho"
    ],
    "rental": [
      "Priya Nair (PM)",
      "Jess Lau (PM)"
    ],
    "conv": [
      "Chen & Co Conveyancing",
      ""
    ]
  },
  "queue": 0,
  "captures": [
    {
      "id": "v1",
      "text": "Spoke to Mary at 12 Gager St, kids moved out, might sell next year. Mobile 0412 555 019.",
      "rec": "9:12am",
      "sync": "9:40am",
      "propId": "p6"
    },
    {
      "id": "v2",
      "text": "2 Gager St, not home, left card.",
      "rec": "9:20am",
      "sync": "9:40am",
      "propId": "p1"
    },
    {
      "id": "v3",
      "text": "Mr Wu at 8 Gager St wants a market update, mobile 0401 777 302.",
      "rec": "9:31am",
      "sync": "9:40am",
      "propId": "p4"
    }
  ],
  "market": [
    {
      "id": "m1",
      "kind": "sale",
      "propId": "p3",
      "addr": "6 Gager St, Sunnybank",
      "meta": "Sold 3 Nov · $1.21m",
      "sub": "Owner Helen Papas · tagged Seller lead (Warm)",
      "owner": "Helen Papas"
    },
    {
      "id": "m2",
      "kind": "listing",
      "propId": "p10",
      "addr": "7 Hill St, Runcorn",
      "meta": "Listed 9 Nov · Ray White",
      "sub": "Owner John Lee · was your Seller lead (Hot)",
      "owner": "John Lee",
      "agency": "Ray White"
    }
  ],
  "patch": {
    "suburbs": [
      "Sunnybank",
      "Runcorn"
    ]
  },
  "myTags": [
    {
      "n": "Cantonese speaking",
      "by": "Owen Chen",
      "d": "2026-10-02"
    },
    {
      "n": "Call after 6pm",
      "by": "Owen Chen",
      "d": "2026-10-14"
    },
    {
      "n": "Spring 2027",
      "by": "Owen Chen",
      "d": "2026-10-20"
    },
    {
      "n": "School catchment",
      "by": "Natalie Wong",
      "d": "2026-10-05"
    }
  ]
})) as AppState;
