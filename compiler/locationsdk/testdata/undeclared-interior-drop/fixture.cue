package negative_interior

// Deliberate missing strip inside the east wall. Ordinary location generation
// must reject this before any mesh is emitted, even without a drop annotation.
sdk: {
 catalog: {
  unit_mm: 500
  geometries: [{id:"empty",width:1,depth:1,height:1,footprint:[],blocked:[],use_clearance:[],maintenance_clearance:[],ports:[]}]
  modules: [{id:"empty.module",geometry_id:"empty",variants:[{id:"neutral",geometry_id:"empty"}]}]
  times: [{id:"day",brightness:1,activity_tags:[]}]
  events: [{id:"normal",activity_tags:[]}]
  decorations: []
 }
 layouts: [{
  id:"negative.interior",unit_mm:500,seed:1,width:4,depth:4,elevation:0,grounded:true
  setting:"dome_interior",interior:{envelope:{x:0,y:0,w:4,h:4},voids:[]}
  floor:[{id:"incomplete.floor",rect:{x:0,y:0,w:3,h:4},reason:"Deliberate gap before the east wall."}]
  entries:[{x:0,y:0}],instances:[],reserved_paths:[],drops:[]
 }]
}
