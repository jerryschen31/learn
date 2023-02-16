import yaml
"""
First, create a configuration file that stores the values of your feature flags. You could use a simple format such as JSON or YAML. Here's an example in YAML format:
In your Python code, load the configuration file and use the values of the feature flags to control the behavior of your code. 
Note that in practice, you would likely want to handle cases where the configuration file is missing or the feature flags are not defined. 
You might also want to implement more sophisticated mechanisms for loading and storing configuration data, such as using environment variables or a database.

"""

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