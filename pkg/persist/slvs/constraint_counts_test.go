package slvs

// constraintFixtureStats is a constraint fixture's number of groups,
// entities and constraints after it loads and regenerates: what the
// loader gives each group, recorded when it was first right.
type constraintFixtureStats struct {
	Groups      int
	Entities    int
	Constraints int
}

var constraintFixtureGoldens = map[string]constraintFixtureStats{
	"angle":              {Groups: 2, Entities: 18, Constraints: 2},
	"arc_line_tangent":   {Groups: 2, Entities: 23, Constraints: 4},
	"comment":            {Groups: 2, Entities: 12, Constraints: 1},
	"cubic_line_tangent": {Groups: 2, Entities: 23, Constraints: 4},
	"diameter":           {Groups: 2, Entities: 16, Constraints: 1},
	"eq_len_pt_line_d":   {Groups: 2, Entities: 19, Constraints: 1},
	"eq_pt_ln_distances": {Groups: 2, Entities: 20, Constraints: 1},
	"equal_angle":        {Groups: 2, Entities: 24, Constraints: 3},
	"equal_length_lines": {Groups: 2, Entities: 18, Constraints: 1},
	"equal_line_arc_len": {Groups: 2, Entities: 20, Constraints: 1},
	"equal_radius":       {Groups: 2, Entities: 20, Constraints: 1},
	"length_difference":  {Groups: 2, Entities: 18, Constraints: 1},
	"length_ratio":       {Groups: 2, Entities: 18, Constraints: 1},
	"parallel":           {Groups: 2, Entities: 18, Constraints: 1},
	"perpendicular":      {Groups: 2, Entities: 18, Constraints: 1},
	"points_coincident":  {Groups: 2, Entities: 14, Constraints: 1},
	"proj_pt_distance":   {Groups: 2, Entities: 14, Constraints: 1},
	"pt_face_distance":   {Groups: 3, Entities: 33, Constraints: 2},
	"pt_in_plane":        {Groups: 2, Entities: 13, Constraints: 1},
	"pt_line_distance":   {Groups: 2, Entities: 16, Constraints: 1},
	"pt_on_circle":       {Groups: 2, Entities: 17, Constraints: 1},
	"pt_on_face":         {Groups: 3, Entities: 33, Constraints: 2},
	"pt_on_line":         {Groups: 2, Entities: 16, Constraints: 1},
	"pt_plane_distance":  {Groups: 2, Entities: 13, Constraints: 1},
	"pt_pt_distance":     {Groups: 2, Entities: 14, Constraints: 1},
	"same_orientation":   {Groups: 2, Entities: 16, Constraints: 1},
	"symmetric":          {Groups: 2, Entities: 14, Constraints: 1},
	"symmetric_horiz":    {Groups: 2, Entities: 14, Constraints: 1},
	"symmetric_line":     {Groups: 2, Entities: 17, Constraints: 1},
	"symmetric_vert":     {Groups: 2, Entities: 14, Constraints: 1},
	"where_dragged":      {Groups: 2, Entities: 13, Constraints: 1},
}
