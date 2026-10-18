package sample

// People who enquire. Names are common across Kenya's communities and among
// the buyers from abroad a Riverside development attracts. They are
// combined at random, so no entry describes a real person.
var firstNames = []string{
	"Achieng", "Amina", "Wanjiru", "Njeri", "Akinyi", "Mumbua", "Chebet", "Nafula", "Wairimu", "Atieno",
	"Halima", "Zawadi", "Nyambura", "Kerubo", "Jebet", "Mwende", "Naliaka", "Adhiambo", "Wambui", "Moraa",
	"Priya", "Fatuma", "Leila", "Grace", "Esther", "Ruth", "Lydia", "Sharon", "Mercy", "Joy",
	"Brian", "Kevin", "Dennis", "Otieno", "Kamau", "Mwangi", "Kiptoo", "Wekesa", "Omondi", "Mutua",
	"Hassan", "Kibet", "Njoroge", "Ochieng", "Kiprono", "Barasa", "Muriuki", "Onyango", "Kariuki", "Ndirangu",
	"Rahul", "Samir", "Daniel", "Peter", "James", "Victor", "Eric", "George", "Martin", "Collins",
}

var lastNames = []string{
	"Odhiambo", "Kamau", "Wanjiku", "Mwangi", "Otieno", "Kiprotich", "Wafula", "Njoroge", "Ochieng", "Mutua",
	"Kariuki", "Achieng", "Kiplagat", "Omondi", "Nyaga", "Gitau", "Chelimo", "Wekesa", "Muthoni", "Onyango",
	"Abdi", "Hassan", "Mohamed", "Shah", "Patel", "Mehta", "Oduya", "Macharia", "Kirui", "Rotich",
	"Ngugi", "Owino", "Kimani", "Barasa", "Simiyu", "Kemboi", "Langat", "Nduta", "Maina", "Wairimu",
}

// emailDomains are the domains reserved for examples, so no address in the
// sample can reach a real person.
var emailDomains = []string{"example.com", "example.com", "example.net", "example.org"}

// Where people heard about Oriel, with how often each happens.
var sources = []struct {
	name   string
	weight int
}{
	{"Website", 34},
	{"Referral", 15},
	{"Property portal", 15},
	{"Social media", 12},
	{"Walk-in", 10},
	{"Agent network", 8},
	{"Site hoarding", 6},
}

var lostReasons = []string{
	"Bought elsewhere",
	"Budget below our prices",
	"Wants a home ready now, not off-plan",
	"No reply after three calls",
	"Prefers Westlands or Kilimani",
	"Mortgage not approved",
	"Plans changed: relocating abroad",
	"Wanted a ground-floor garden home",
}

// Messages people leave when they first enquire, by what they are after.
var enquiryMessages = map[string][]string{
	"A": {
		"Looking at a one bedroom as an investment to let. What rental yield are owners seeing in Riverside?",
		"Interested in a 1 bed on a middle floor. Is parking included?",
		"I'm buying my first home and like the one bedrooms. Can I pay the deposit in two parts?",
		"Do the one bedrooms facing the river cost much more than the south side?",
	},
	"B": {
		"We are a couple looking for a 2 bed with good light. Which corner gets the morning sun?",
		"Interested in a two bedroom on a high floor. Is the cash discount still available?",
		"Living in Dubai, looking for a 2 bed to move back to once it is finished. Can I do everything remotely?",
		"Please send the price list for two bedrooms and the payment plan details.",
	},
	"C": {
		"Family of four, interested in a 3 bed facing the river. When is completion?",
		"Looking for three bedrooms with a big living room. Can we see the show apartment on Saturday?",
		"Is there a 3 bed left on floors 10 and above?",
	},
	"D": {
		"We need three bedrooms and a DSQ. What is the size of the staff room?",
		"Interested in the north-east corner homes with the study. Are any still available?",
		"Please share the floor plan for the 3 bed with study and DSQ.",
	},
	"P": {
		"Interested in the penthouses. Can I arrange a private viewing?",
		"Please send details of the top-floor homes and the terrace sizes.",
	},
}

var callNotes = []string{
	"Called and talked through the payment plans. Sent the price list on WhatsApp.",
	"Spoke for ten minutes. Wants to see the show apartment before deciding.",
	"Called back as promised. Comparing us with two other projects.",
	"Left a voicemail, then spoke in the evening. Keen on a high floor.",
	"Long call about the construction schedule and the bank we work with.",
	"Explained the reservation steps. Will bring their spouse to the viewing.",
}

var viewingNotes = []string{
	"Loved the light in the corner bay. Asked about upper floors.",
	"Walked both show apartments. Wants to think it over with family.",
	"Measured the kitchen. Asked about finishes and the appliance package.",
	"Came with their architect. Very positive about the plan.",
	"Short visit. Price is at the top of their budget.",
	"Asked a lot about the payment plan; wants the schedule by email.",
	"Brought their parents. Keen on a river-side home.",
	"Liked the bay but wants a second bedroom; showed them type B.",
	"Comparing with a project in Kileleshwa. Will decide this month.",
	"Investor buying to let. Asked about rental demand and service charge.",
	"Sat in the bay for ten minutes. Very taken with it.",
	"Wants a high floor facing the Arboretum. Sent the stacking plan.",
	"Asked about parking for two cars and charging points.",
	"Mortgage already approved in principle. Ready to reserve.",
	"Worried about completion dates; walked them through the programme.",
	"Visiting from the UK this week only. Will reserve before flying back.",
}
