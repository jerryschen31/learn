import yaml

with open("config.yml", "r") as f:
    config = yaml.safe_load(f)

if config["feature_flags"]["new_feature_enabled"]:
    # Run the new feature code
    print("This is the new feature!")
else:
    # Run the old code
    print("This is the old code.")
    
if config["feature_flags"]["feature_x_enabled"]:
    # Run feature X code
    print("This is feature X!")
    
if config["feature_flags"]["feature_y_enabled"]:
    # Run feature Y code
    print("This is feature Y!")