extends SceneTree

var errors: Array[String] = []

func _initialize() -> void:
	call_deferred("verify")

func check(ok: bool, message: String) -> void:
	if not ok:
		errors.append(message)

func vec(values: Array) -> Vector3:
	return Vector3(values[0], values[1], values[2])

func verify() -> void:
	var expected = JSON.parse_string(FileAccess.get_file_as_string("res://oracle.json"))
	var manifest = JSON.parse_string(FileAccess.get_file_as_string("res://placements.json"))
	var models = JSON.parse_string(FileAccess.get_file_as_string("res://models.json"))
	var packed = load("res://location.tscn") as PackedScene
	if packed == null:
		push_error("generated scene did not load")
		quit(1)
		return
	var scene = packed.instantiate()
	root.add_child(scene)
	var placements = scene.get_node("Placements")
	check(placements.get_child_count() == expected.instances.size(), "placement count")
	var max_position_error := 0.0
	var max_basis_error := 0.0
	var nodes: Dictionary = {}
	var model_count := 0
	var shared_resources: Dictionary = {}
	for node in placements.get_children():
		var id: String = node.get_meta("instance_id")
		nodes[id] = node
		check(expected.instances.has(id), "unexpected ID " + id)
		if not expected.instances.has(id):
			continue
		var row = expected.instances[id]
		var t: Transform3D = node.global_transform
		var delta := t.origin.distance_to(vec(row.position))
		max_position_error = maxf(max_position_error, delta)
		check(delta < 0.00002, "world position " + id)
		for axis in range(3):
			var basis_delta := t.basis[axis].distance_to(vec(row.basis_columns[axis]))
			max_basis_error = maxf(max_basis_error, basis_delta)
			check(basis_delta < 0.00002, "world basis " + id)
		var model_ref: String = manifest.instances[id].model_ref
		if model_ref != "":
			model_count += 1
			check(node.scene_file_path == models[model_ref], "model resource " + id)
			shared_resources[model_ref] = true
	var camera: Camera3D = scene.get_node("Camera3D")
	check(camera.projection == Camera3D.PROJECTION_ORTHOGONAL, "orthographic camera")
	check(camera.get_meta("elevation_degrees") == 35, "camera elevation")
	check(camera.get_meta("azimuth_degrees") == 37, "camera azimuth")
	check(absf(camera.size - 49) < 0.00001, "camera framing")
	check(absf(camera.global_basis.determinant() - 1) < 0.00001, "proper camera basis")
	var focus := Vector3(28.5, 4.6, -22)
	var looks_at_focus := camera.global_basis.z.distance_to((camera.global_position - focus).normalized()) < 0.00001
	check(looks_at_focus, "camera must look at the authored focus")
	var crane: Node3D = nodes["crane.bridge"]
	var crane_screen_right := (crane.global_position - focus).dot(camera.global_basis.x)
	check(crane.global_position.x < focus.x and crane_screen_right > 0, "west crane must project screen-right")
	# The train root is its leading end. Check the actual imported model centre,
	# since the four-car body extends west from that root into the lower-right.
	var train: Node3D = nodes["train.c7.passenger.01"]
	var points: Array[Vector3] = []
	mesh_corners(train, points)
	check(not points.is_empty(), "train contains real imported meshes")
	var train_screen := Vector2.ZERO
	var train_bounds: Dictionary = {}
	if not points.is_empty():
		var lo := points[0]
		var hi := points[0]
		for point in points:
			lo = lo.min(point)
			hi = hi.max(point)
		var centre := (lo + hi) * 0.5 - focus
		train_bounds = {"min": [lo.x, lo.y, lo.z], "max": [hi.x, hi.y, hi.z], "mesh_corners": points.size()}
		train_screen = Vector2(centre.dot(camera.global_basis.x), -centre.dot(camera.global_basis.y))
		check(train_screen.x > 0 and train_screen.y > 0, "train model must project lower-right")
	var report := {
		"passed": errors.is_empty(), "godot_version": Engine.get_version_info().string,
		"placements_loaded": placements.get_child_count(), "model_instances": model_count,
		"shared_model_resources": shared_resources.size(),
		"max_position_error_m": max_position_error, "max_basis_error": max_basis_error,
		"crane_west_screen_right": crane_screen_right > 0,
		"camera_looks_at_focus": looks_at_focus,
		"train_lower_right": train_screen.x > 0 and train_screen.y > 0,
		"train_screen_offset_m": [train_screen.x, train_screen.y],
		"train_world_bounds": train_bounds,
		"structure_meshes": scene.get_node("Structure").get_child_count(), "errors": errors,
	}
	var file := FileAccess.open("res://scene-load-verification.json", FileAccess.WRITE)
	file.store_string(JSON.stringify(report, "  ") + "\n")
	file.close()
	print(JSON.stringify(report))
	quit(0 if errors.is_empty() else 1)

func mesh_corners(node: Node, points: Array[Vector3]) -> void:
	if node is MeshInstance3D:
		var bounds: AABB = node.get_aabb()
		for index in range(8):
			points.append(node.global_transform * bounds.get_endpoint(index))
	for child in node.get_children():
		mesh_corners(child, points)
