package fixture

first: {
 compiled: {
  life: {locations:room:{cell_mm:500,terrain:["#####","#...#","#...#","#...#","#####"]},edges:[{id:"leave",from:"room",to:"other",exit:{x:1,y:2}}]}
  walkable:{".":true}
  localization:messages:[{key:"ui.zone",context:"Destination of this fixture exit.",texts:{ru:"Выход",en:"Exit"}}]
  modular_locations:[{level_id:"room",layout_id:"first.layout"}]
  location_sdk:levels:[{id:"first.layout",unit_mm:500,ports:[{id:"door",instance_id:"entry",kind:"connection",at:{x:0,y:2},approach:[{x:1,y:2},{x:1,y:3}]}]}]
 }
 world:exit_zones:zones:[{edge_id:"leave",kind:"walk",text_key:"ui.zone",destination_key:"ui.zone",derive_radius_cells:1}]
}
second: {
 compiled: {
  life: {locations:second:{cell_mm:500,terrain:["######","#....#","#....#","#....#","######"]},edges:[{id:"second_exit",from:"second",to:"another",exit:{x:4,y:2}}]}
  walkable:{".":true}
  localization:messages:[{key:"ui.other_zone",context:"Destination of the second fixture exit.",texts:{ru:"Другой выход",en:"Another exit"}}]
  modular_locations:[{level_id:"second",layout_id:"second.layout"}]
  location_sdk:levels:[{id:"second.layout",unit_mm:500,ports:[{id:"east",instance_id:"doorway",kind:"connection",at:{x:5,y:2},approach:[{x:4,y:1},{x:4,y:2}]}]}]
 }
 world:exit_zones:zones:[{edge_id:"second_exit",kind:"walk",text_key:"ui.other_zone",destination_key:"ui.other_zone",derive_radius_cells:1}]
}
