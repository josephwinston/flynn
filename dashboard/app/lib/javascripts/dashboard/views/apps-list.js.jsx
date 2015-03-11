//= require ./route-link
//= require ./external-link
//= require ../stores/app

(function () {

"use strict";

var RouteLink = Dashboard.Views.RouteLink;
var isSystemApp = Dashboard.Stores.App.isSystemApp;

Dashboard.Views.AppsList = React.createClass({
	displayName: "Views.AppsList",

	render: function () {
		var apps = this.state.apps;

		var getAppPath = this.props.getAppPath;
		var selectedAppId = this.props.selectedAppId;

		return (
			<ul className="apps-list">
				{apps.map(function (app) {
					return (
						<li key={app.id} className={Marbles.Utils.assertEqual(app.id, selectedAppId) ? "selected" : ""}>
							<RouteLink path={getAppPath(app.id)}>
								{app.name}
							</RouteLink>
						</li>
					);
				}.bind(this))}
			</ul>
		);
	},

	getDefaultProps: function () {
		return {
			apps: [],
			getAppPath: function (appId) {
				return Marbles.history.pathWithParams("/apps/:id", [{ id: appId }]);
			}
		};
	},

	componentWillMount: function () {
		this.setState(this.__getState(this.props));
	},

	componentWillReceiveProps: function (props) {
		this.setState(this.__getState(props));
	},

	__getState: function (props) {
		var state = {};

		var showSystemApps = props.showSystemApps;
		state.apps = props.apps.filter(function (app) {
			return !isSystemApp(app) || showSystemApps;
		});

		return state;
	}
});

})();
